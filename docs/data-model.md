# Data Model

Schema is managed by [goose](https://github.com/pressly/goose) migrations in `internal/db/migrations/`, applied automatically at startup (`goose.Up` in `main.go`) or manually via `make migrate-up`. Conventions (see `CLAUDE.md`): `NNN_description.sql` naming, `-- +goose Up` / `-- +goose Down` sections, every migration reversible, one table per file, named constraints.

Requires PostgreSQL 16 with the `pgvector` extension (migration 005); the Docker image is `pgvector/pgvector:pg16`.

Tables marked **RLS** carry the `tenant_isolation_*` policy with `FORCE ROW LEVEL SECURITY` — see [multi-tenancy.md](multi-tenancy.md) for the policy pattern. Tables marked **Global Directory** are deliberately unprotected because they are read before a tenant context exists.

## organizations — Global Directory (008)

| Column | Type | Notes |
|---|---|---|
| id | UUID PK | `gen_random_uuid()` |
| name | TEXT NOT NULL | `''` when not inferable at provisioning; admins set it later via `PATCH /api/v1/org` |
| created_at | TIMESTAMPTZ NOT NULL | |

## platform_workspaces — Global Directory (009, bot token added in 016)

Maps an external workspace/tenant identifier to exactly one organization, and stores the workspace-scoped bot token obtained via the Slack OAuth install flow.

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| org_id | UUID NOT NULL | `fk_platform_workspaces_org` → organizations, CASCADE |
| provider | TEXT NOT NULL | OIDC issuer URL, e.g. `https://slack.com` |
| external_id | TEXT NOT NULL | Slack `team_id`; future Teams/SAML tenant ID |
| bot_token | TEXT NULL | Workspace-scoped Slack bot token (`xoxb-…`); written by `GET /api/slack/callback` after a successful OAuth exchange. NULL until the install flow completes. The Events API webhook functions without it; outbound Slack API calls require it. |
| team_name | TEXT NULL | Slack workspace display name; written alongside `bot_token`. |
| created_at | TIMESTAMPTZ NOT NULL | |

Constraints: `uq_platform_workspaces_provider_external UNIQUE (provider, external_id)` — the anchor for the concurrent-provisioning race in `ClaimWorkspace`. Index: `idx_platform_workspaces_org`.

**Token vs. signing secret:** `bot_token` is per-workspace (unique per install). The signing secret used to verify incoming webhook requests is per-app (same for all workspaces) and lives in `SLACK_SIGNING_SECRET` env var — it is never stored in the database.

## users — RLS (001, tenancy added in 011)

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| email | TEXT NOT NULL | unique **per org**: `uq_users_org_email UNIQUE (org_id, email)` (011 dropped the global `uq_users_email` — the same person may exist in two orgs) |
| name | TEXT NOT NULL DEFAULT '' | |
| org_id | UUID NULL | `fk_users_org` → organizations, CASCADE. NULL only on legacy rows — invisible in pooled mode |
| team_id | UUID NULL | `fk_users_team` → teams, **ON DELETE SET NULL** (deleting a team unassigns its members) |
| role | VARCHAR(20) NOT NULL DEFAULT 'UPDATER' | `ck_users_role CHECK (role IN ('ADMIN','TEAM_EDITOR','UPDATER'))` |
| created_at / updated_at | TIMESTAMPTZ NOT NULL | |

Indexes: `idx_users_org`, `idx_users_team`. Policy: `tenant_isolation_users`.

## user_identities — Global Directory (002, org added in 012)

External login identity → internal user resolution.

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| user_id | BIGINT NOT NULL | → users, CASCADE |
| provider | TEXT NOT NULL | OIDC issuer URL |
| provider_id | TEXT NOT NULL | provider `sub` claim (Slack user ID) |
| org_id | UUID NULL | `fk_user_identities_org` → organizations, CASCADE; backfilled on upsert for legacy rows |
| created_at | TIMESTAMPTZ NOT NULL | |

Constraints: `uq_user_identities_provider UNIQUE (provider, provider_id)`. Index: `idx_user_identities_lookup (provider, provider_id)`.

## sessions — Global Directory (003, org added in 013)

The bootstrap row that tells the API middleware which tenant to bind.

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| user_id | BIGINT NOT NULL | → users, CASCADE |
| token | TEXT NOT NULL | opaque 64-hex-char value; `uq_sessions_token UNIQUE` |
| org_id | UUID NULL | `fk_sessions_org` → organizations, CASCADE; NULL only on legacy sessions (rejected in pooled mode) |
| created_at | TIMESTAMPTZ NOT NULL | |
| expires_at | TIMESTAMPTZ NOT NULL | default `NOW() + 30 days`; `idx_sessions_expiry` |

## teams — RLS (010)

| Column | Type | Notes |
|---|---|---|
| id | UUID PK | `gen_random_uuid()` |
| org_id | UUID NOT NULL | `fk_teams_org` → organizations, CASCADE |
| name | TEXT NOT NULL | |
| created_at | TIMESTAMPTZ NOT NULL | |

Index: `idx_teams_org`. Policy: `tenant_isolation_teams`. Note: `org_id` is NOT NULL here (the table was born tenant-aware), unlike the retrofitted tables.

## daily_updates — RLS (004, embedding 006, org added in 014)

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| user_id | BIGINT NOT NULL | `fk_daily_updates_user` → users, CASCADE |
| org_id | UUID NULL | `fk_daily_updates_org` → organizations, CASCADE |
| update_text | TEXT NOT NULL | raw message text from Slack |
| update_embedding | vector(384) NULL | all-MiniLM-L6-v2 L2-normalised embedding; written asynchronously by NLPWorker |
| created_at / updated_at | TIMESTAMPTZ NOT NULL | |
| deleted_at | TIMESTAMPTZ NULL | soft delete; all dashboard queries filter `deleted_at IS NULL` |

Indexes: `idx_daily_updates_user_active (user_id, created_at DESC) WHERE deleted_at IS NULL`, `idx_daily_updates_org (org_id, created_at DESC) WHERE deleted_at IS NULL`. Policy: `tenant_isolation_daily_updates`.

## daily_update_topics — RLS (007, org added in 015)

Noun-phrase topics extracted from an update by the NLP pipeline.

| Column | Type | Notes |
|---|---|---|
| id | BIGSERIAL PK | |
| daily_update_id | BIGINT NOT NULL | `fk_daily_update_topics_update` → daily_updates, CASCADE |
| org_id | UUID NULL | `fk_daily_update_topics_org` → organizations, CASCADE |
| extracted_topic | TEXT NOT NULL | lemmatised "verb object" phrase from the spaCy sidecar |
| topic_embedding | vector(384) NULL | per-phrase embedding |
| created_at | TIMESTAMPTZ NOT NULL | |

Indexes: `idx_daily_update_topics_update_id`, `idx_daily_update_topics_org`, and `idx_daily_update_topics_embedding` — ivfflat with `vector_cosine_ops` (`lists = 10`) partial on `topic_embedding IS NOT NULL`, backing the `<=>` cosine-distance dashboard query. Policy: `tenant_isolation_daily_update_topics`.

## River tables

The River job queue manages its own schema (`river_job`, `river_migration`, …) via `rivermigrate` at startup; it is not part of the goose migration chain. Job rows embed `jobs.NLPProcessingArgs` (`daily_update_id`, `org_id`) as JSON.
