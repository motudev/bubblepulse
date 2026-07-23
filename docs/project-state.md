# BubblePulse — Project State

## Architecture

BubblePulse is a single Go binary (`cmd/bubblepulse`) that runs the HTTP server and a River background worker in the same process, backed by PostgreSQL and a Python NLP sidecar.

```
                   ┌──────────────────────────────────────────────┐
                   │              Go backend (:8080)              │
                   │                                              │
 Browser (Vue SPA)►│  internal/api      HTTP handlers + RBAC      │
                   │  internal/auth     OIDC login (Slack)        │
 Slack Events API ►│  internal/platform/slack  webhook adapter    │
                   │        │                                     │
                   │  internal/messaging  provider-agnostic       │
                   │        │             ingestion service       │
                   │        ▼                                     │
                   │  River queue ──► internal/worker  NLPWorker  │
                   │                       │        │             │
                   └───────────────────────┼────────┼─────────────┘
                                           ▼        ▼
                                   PostgreSQL 16   Python spaCy
                                   + pgvector      sidecar (:8090)
                                   + RLS policies  noun phrases
```

### Package responsibilities

| Package | Responsibility |
|---|---|
| `cmd/bubblepulse` | Composition root — wires config, DB, migrations, tenancy runner, River, OIDC, repositories, and handlers. The only `main` package. |
| `internal/api` | HTTP mux, session/role middleware, and all JSON handlers. Defines small consumer-side interfaces for each dependency. |
| `internal/auth` | OIDC login / callback / logout, org resolution and auto-provisioning, session cookie issuance. |
| `internal/tenancy` | Tenant-context plumbing: context propagation, the `Runner` that binds every transaction to a tenant GUC (`app.current_org_id`), startup RLS safety check. |
| `internal/db/repository` | pgx-backed repositories for users, sessions, orgs, teams, workspaces, and daily updates. |
| `internal/messaging` | Provider-agnostic ingestion: resolves an incoming message's identity and org, writes the update row, enqueues the NLP job — all in one atomic transaction. |
| `internal/platform/slack` | Slack Events API adapter: HMAC signature verification, URL-verification handshake, DM extraction. Additional platforms implement `messaging.PlatformAdapter` and are registered in `main.go`. |
| `internal/worker` | River `NLPWorker` — generates 384-dim sentence embeddings (all-MiniLM-L6-v2, in-process via ONNX) and calls the spaCy sidecar for topic phrases. |
| `internal/jobs` | River job argument types. |
| `pkg/config` | Environment-variable loading and validation, collecting all missing keys before returning an error. |
| `nlp_service/` | Python FastAPI sidecar: `POST /parse` extracts lemmatised noun phrases via spaCy dependency parsing. |
| `frontend/` | Vue 3 + Vite + TypeScript + Pinia SPA. |

### Ingestion pipeline

1. Slack posts to `POST /api/slack/events`. The adapter verifies the `X-Slack-Signature` HMAC and acknowledges immediately with 200.
2. For DM messages the adapter calls `MessageService.Handle` with a normalised `IncomingMessage`.
3. `MessageService` resolves the user identity (Global Directory via `user_identities`), resolves the org (falling back to `platform_workspaces` by Slack `team_id`), then opens one tenant-scoped transaction that inserts the `daily_updates` row and enqueues a River job — atomically.
4. `NLPWorker` picks up the job: computes the 384-dim embedding in-process, calls the sidecar for noun phrases, embeds each phrase, and writes embedding + topics to `daily_update_topics` in a tenant-scoped write transaction.

### Multi-tenancy

Two modes, toggled by `TENANCY_MODE`:

- **Pooled** (default, SaaS): all tenants share one DB role. Every transaction has `app.current_org_id` set as a session GUC. RLS policies on each table enforce per-org isolation. A startup check (`tenancy.VerifyPooledSafety`) refuses to start if the DB role has `BYPASSRLS` or is a superuser.
- **Siloed** (single-tenant self-hosted): one DB per org; RLS is bypassed. The `tenancy.Runner` still wraps transactions but does not set the GUC.

### Frontend

Three routes: `/` (login), `/dashboard` (session required), `/admin` (ADMIN or TEAM\_EDITOR role). A router guard resolves the session once via `/api/v1/me` and caches it. All HTTP goes through `src/services/api.ts`. The `scope` Pinia store toggles the dashboard between org-wide and the user's own team. `BubbleMap.vue` uses a D3 force simulation to render users and topics as nodes; edges connect topics whose pairwise cosine similarity exceeds 0.85. Role checks in the UI are convenience gates only — the backend enforces the same rules authoritatively.

---

## Current State vs. Project Goals

### Phase 1 — Core Loop (MVP)

**Solid:**
- **Slack Events API** is wired end-to-end — signature verification, DM parsing, and daily update insertion all work via `internal/platform/slack` and `internal/messaging`.
- **Go REST API** is production-quality: OIDC auth, session middleware, role-based access (`adminOnly`, `adminOrEditor`), multi-tenancy with RLS, and correct `pgx/v5` usage throughout.
- **PostgreSQL schema** has 15 migrations: users, identities, sessions, daily updates, orgs, teams, workspaces, and vector embeddings.
- **Bubble Map** (`BubbleMap.vue`) is functional — a D3 force-directed SVG with user nodes, topic nodes, similarity-weighted edges, pan/zoom, hover tooltips, and a demo mode.

**Gaps:**
- The vision calls for **Focus / Friction / Energy as three distinct fields**, but `daily_updates` stores free-form text only — no structured energy or sentiment columns yet.
- **Blocker/dependency edges between people** (the "red line between UI Designer and API Dev" use case from the vision) do not exist. The graph connects users to shared topics, not users to each other.
- **Slack slash commands and modal input** are not implemented — only DM text is ingested today.

### Phase 2 — Smart Routing & Subscriptions

**Solid foundation:**
- pgvector embeddings are stored per update (migrations 005–006), topic extraction runs as a River background job, and the dashboard API computes a full topic-similarity matrix that drives the Bubble Map edges.
- The data pipeline from "Slack DM received" through "embedded + topics extracted" is complete.

**Not started:**
- **Topic subscriptions** — no mechanism for a PO or lead to subscribe to a keyword and receive a notification when someone's update matches.
- **Automated blocker-edge creation** from natural language parsing.

### Phase 3 — Team Health & Analytics

Not started. No energy-level scoring, sentiment aggregation, async retro check-ins, calendar integrations, or information-flow analysis exist yet.

### Summary

The project is in **late Phase 1 / early Phase 2**. The infrastructure layer — auth, multi-tenancy, job queue, NLP pipeline — is well ahead of the product-facing features. The plumbing is solid; the remaining Phase 1 work is the structured Focus/Friction/Energy fields, person-to-person blocker edges, and the Slack modal input. Phase 2's subscription and routing features come after that.
