# Architecture

BubblePulse is a self-hosted asynchronous check-in platform: team members post daily updates via Slack DM, an NLP pipeline extracts topics and embeddings, and a Vue SPA renders the team as a force-directed "bubble map" showing who is working on what — and whose work overlaps.

## System overview

```
                       ┌──────────────────────────────────────────────┐
                       │                Go backend (:8080)            │
                       │                                              │
 Browser (Vue SPA) ───►│  internal/api      HTTP handlers + RBAC      │
                       │  internal/auth     OIDC login (Slack)        │
 Slack Events API ────►│  internal/platform/slack  webhook adapter    │
                       │        │                                     │
                       │  internal/messaging  provider-agnostic       │
                       │        │             ingestion service       │
                       │        ▼                                     │
                       │  River queue ──► internal/worker  NLPWorker  │
                       │        │              │        │             │
                       └────────┼──────────────┼────────┼─────────────┘
                                ▼              ▼        ▼
                        PostgreSQL 16     all-MiniLM   Python spaCy
                        + pgvector        (ONNX, in-   sidecar (:8090)
                        + RLS policies    process)     noun phrases
```

Everything is one Go binary (`cmd/bubblepulse`) plus two containers (Postgres, the Python NLP sidecar) and the Vite-built SPA.

## Package map

| Package | Responsibility |
|---|---|
| `cmd/bubblepulse` | Composition root — the only `main` package. Wires config, DB, migrations, tenancy runner, River, OIDC, repositories, handlers. |
| `internal/api` | HTTP mux, middleware (`requireSession`, `requireRole`), and all JSON handlers. Defines the consumer-side interfaces it needs (`SessionLookup`, `UserStore`, `TeamStore`, `OrgStore`, `tenantTxRunner`, `dashboardQuerier`). |
| `internal/auth` | OIDC login/callback/logout, org resolution and auto-provisioning, session cookie issuance. |
| `internal/tenancy` | Tenant-context plumbing: context propagation, the `Runner` that binds transactions to a tenant GUC, the startup RLS safety check, UUID validation. See [multi-tenancy.md](multi-tenancy.md). |
| `internal/db` | `pgxpool` connection helper; goose SQL migrations in `internal/db/migrations`. |
| `internal/db/repository` | pgx-backed repositories (`UserRepo`, `TeamRepo`, `OrgRepo`, `SessionRepo`, `WorkspaceRepo`, `DailyUpdateRepo`), the shared `Querier` interface, role constants. |
| `internal/messaging` | Provider-agnostic ingestion: resolves an incoming message's identity and org, writes the update, enqueues the NLP job — all in one transaction. |
| `internal/platform/slack` | Slack Events API adapter: signature verification, URL-verification handshake, DM message extraction. New platforms implement `messaging.PlatformAdapter` and get registered in `main.go`. |
| `internal/worker` | River `NLPWorker` (embeddings + topic extraction) and the HTTP client for the spaCy sidecar. |
| `internal/jobs` | River job argument types (`NLPProcessingArgs`). |
| `pkg/config` | Environment configuration loading and validation. |
| `nlp_service/` | Python FastAPI sidecar: `POST /parse` extracts lemmatised "verb object" noun phrases via spaCy dependency parsing. |
| `frontend/` | Vue 3 + Vite + TypeScript + Pinia SPA. |

## Startup order (`cmd/bubblepulse/main.go`)

Each step is fatal on error:

1. `godotenv.Load()` — loads `.env` if present (no-op in production).
2. `config.Load()` — validates env vars, collecting **all** missing keys into one error.
3. `db.Connect()` — opens the `pgxpool` against `DATABASE_URL`.
4. `goose.Up("internal/db/migrations")` — applies schema migrations.
5. `tenancy.NewRunner(pool, siloed)`; in pooled mode `tenancy.VerifyPooledSafety` refuses to start if the DB role is a superuser or has `BYPASSRLS` (either would silently disable tenant isolation).
6. `rivermigrate.Migrate(DirectionUp)` — River's own schema (idempotent).
7. `allminilm.NewModel` — loads the ONNX all-MiniLM-L6-v2 sentence transformer (requires CGO and `libonnxruntime.so`); wrapped so vectors are always L2-normalised.
8. River worker registration (`NLPWorker`) and client (default queue, `MaxWorkers: 4`).
9. `auth.NewProvider` — OIDC discovery against `OIDC_ISSUER_URL`.
10. Repository construction and `auth.NewHandler` / `messaging.NewMessageService` / platform adapters / `api.New`.
11. `http.Server` on `:PORT` (read 10s / write 30s / idle 120s timeouts).

Graceful shutdown on `SIGINT`/`SIGTERM`: River client stop and HTTP server shutdown, each with a 10-second deadline; pool and embedder closed via `defer`.

## The ingestion pipeline

1. Slack sends an Events API POST to `/api/slack/events`. The adapter verifies the `X-Slack-Signature` HMAC (constant-time compare), answers `url_verification` challenges, and acks `event_callback` with 200 immediately (Slack retries non-2xx).
2. For DM messages (`channel_type == "im"`, non-bot, non-empty), the adapter calls `MessageService.Handle` with `{provider, platform_user_id, workspace_id, text}`.
3. `MessageService.Handle` resolves the identity via the Global Directory (`user_identities`), resolves the org (identity's stored org, falling back to `platform_workspaces` by Slack `team_id`, backfilling the identity), then opens **one tenant-scoped transaction** that inserts the `daily_updates` row and enqueues the River job — atomically.
4. `NLPWorker` picks up the job: reads the text (tenant-scoped read transaction), computes the 384-dim update embedding in-process, calls the sidecar's `POST /parse` for noun phrases, embeds each phrase, then writes embedding + topics in a tenant-scoped write transaction. Topic-extraction failure is non-fatal (embedding is still stored); a job with an empty `org_id` (legacy) is cancelled, not retried.

## Slack OAuth install flow

When `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` are set, two additional routes are registered:

- `GET /api/slack/install` — generates a CSRF state token (stored as a short-lived `slack_install_state` HTTP-only cookie) and redirects to `https://slack.com/oauth/v2/authorize`.
- `GET /api/slack/callback` — verifies the state, exchanges the code at `https://slack.com/api/oauth.v2.access`, provisions or joins the organization (same race-safe `ClaimWorkspace` path used during OIDC login), and stores the workspace's bot token in `platform_workspaces.bot_token`.

**Signing secret vs. bot token:** The signing secret (`SLACK_SIGNING_SECRET`) is per-app — it is the same value for every workspace that installs the app and is legitimately kept in an env var. Bot tokens (`xoxb-…`) are per-workspace and are stored in the database after installation. In siloed (single-tenant) mode, `SLACK_BOT_TOKEN` can be set as a fallback for deployments that do not use the OAuth install flow.

## Configuration (`pkg/config/config.go`)

| Env var | Default | Required | Purpose |
|---|---|---|---|
| `PORT` | `8080` | no | HTTP listen port |
| `TENANCY_MODE` | `pooled` | no | `pooled` (shared SaaS, RLS enforced) or `siloed` (dedicated single-tenant, RLS bypass active) |
| `DATABASE_URL` | — | **yes** | Postgres DSN. In pooled mode the role must be non-superuser / non-`BYPASSRLS` |
| `OIDC_ISSUER_URL` | `https://slack.com` | no | OIDC issuer for login |
| `OIDC_CLIENT_ID` | — | **yes** | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | **yes** | OIDC client secret |
| `OIDC_REDIRECT_URL` | — | **yes** | Callback URL registered with the provider |
| `FRONTEND_URL` | `""` | no | SPA origin for post-login/logout redirects; empty = same origin |
| `SLACK_SIGNING_SECRET` | — | **yes** | Verifies every Slack Events API request via HMAC-SHA256; per-app, shared across all workspace installs |
| `SLACK_BOT_TOKEN` | — | no | Siloed-mode fallback bot token; has no effect in pooled mode (per-workspace tokens come from `platform_workspaces.bot_token`) |
| `SLACK_CLIENT_ID` | — | no | Slack app client ID; required to enable `GET /api/slack/install` and `GET /api/slack/callback` |
| `SLACK_CLIENT_SECRET` | — | no | Slack app client secret; required alongside `SLACK_CLIENT_ID` |
| `SLACK_INSTALL_REDIRECT_URL` | — | no | Redirect URI registered in Slack app settings, pointing to `GET /api/slack/callback` |
| `ONNX_RUNTIME_PATH` | `libonnxruntime.so` | no | Path to the ONNX runtime shared library |
| `NLP_SERVICE_URL` | `http://localhost:8090` | no | Base URL of the spaCy sidecar |

`TEST_DATABASE_URL` is used only by the integration test suite (see [testing.md](testing.md)); `config.Load` never reads it.

## Frontend

Three routes: `/` (login), `/dashboard` (auth required), `/admin` (ADMIN or TEAM_EDITOR). A router guard resolves the session once per app load (`userStore.ensureSession()` caches the in-flight `/api/v1/me` promise). All HTTP goes through `src/services/api.ts`; 401 responses redirect to `/`. The `scope` Pinia store toggles the dashboard between org-wide and the user's own team; `BubbleMap.vue` renders users and topics as a d3-force simulation, linking topics whose pairwise cosine similarity exceeds 0.85. Frontend role checks (admin views, team-edit gates) are UI conveniences only — the backend enforces the same rules authoritatively.
