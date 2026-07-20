# Multi-Tenancy

BubblePulse runs the same schema in two deployment modes, selected by `TENANCY_MODE`:

- **pooled** (default) — one shared database serves many organizations; Postgres row-level security (RLS) isolates tenants.
- **siloed** — a dedicated single-tenant deployment; every RLS policy contains a bypass condition that this mode activates, so the same binaries and migrations run unchanged.

## Tenancy model

```
organizations ──┬── platform_workspaces   (1:N, Global Directory)
                ├── teams                 (1:N, RLS)
                └── users                 (1:N, RLS)
teams ─────────── users                   (1:N, nullable FK, ON DELETE SET NULL)
users ─────────── daily_updates           (1:N, RLS)
daily_updates ─── daily_update_topics     (1:N, RLS)
```

An **organization** is the tenant. A **platform workspace** maps an external identifier (Slack `team_id`; later a Teams/SAML tenant ID) to exactly one org — `UNIQUE (provider, external_id)`. **Teams** subdivide an org for dashboard scoping and delegated administration. Every user carries a `role`: `ADMIN`, `TEAM_EDITOR`, or `UPDATER` (constants in `internal/db/repository/role.go`).

## Global Directory vs RLS-protected tables

The schema splits into two classes, and the distinction drives everything else:

| Class | Tables | Why |
|---|---|---|
| **Global Directory** (no RLS) | `organizations`, `platform_workspaces`, `user_identities`, `sessions` | These rows are how the system *discovers* which tenant a request belongs to — they must be readable **before** any tenant context exists (session-cookie lookup, Slack webhook resolution, OIDC callback). |
| **RLS-protected** (`FORCE ROW LEVEL SECURITY`) | `users`, `teams`, `daily_updates`, `daily_update_topics` | All tenant-owned data. Every read and write is scoped by policy, not by hand-written `WHERE org_id = …` clauses. |

`FORCE` matters: goose runs migrations as the application role, so the application role *owns* the tables, and table owners are exempt from RLS unless it is forced.

## The RLS policy pattern

Every protected table carries the same policy (e.g. `tenant_isolation_teams` in migration `010_create_teams.sql`):

```sql
USING (
    current_setting('app.is_siloed', true) = 'true'
    OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
)
WITH CHECK ( -- same expression )
```

Properties:

- **Fail-closed.** If neither GUC is set, `current_setting(..., true)` returns NULL/empty, `NULLIF` yields NULL, and `org_id = NULL` is never true — the table appears empty. A forgotten tenant binding produces *no data*, never *someone else's data*.
- **Rows with `org_id IS NULL` are invisible in pooled mode** for the same reason (`NULL = uuid` is not true). Legacy pre-tenancy rows only surface in siloed mode.
- `WITH CHECK` blocks inserting or updating rows into another tenant: an `INSERT` whose `org_id` doesn't match the bound tenant is rejected by Postgres.

## `tenancy.Runner` — the single enforcement point

All access to RLS tables goes through `(*tenancy.Runner).RunTx` (`internal/tenancy/runner.go`):

```go
err := runner.RunTx(ctx, func(tx pgx.Tx) error {
    users, err := userRepo.ListByOrg(ctx, tx)  // RLS scopes the query
    ...
})
```

`RunTx` begins a transaction and, before invoking the callback:

- **pooled**: reads the org ID from `ctx` (`tenancy.TenantIDFromContext`); returns `tenancy.ErrNoTenant` *before touching the database* if absent; otherwise executes `SELECT set_config('app.current_tenant_id', $1, true)`.
- **siloed**: executes `SELECT set_config('app.is_siloed', 'true', true)`.

The third `set_config` argument (`is_local = true`) makes the setting **transaction-local**: Postgres discards it on commit or rollback, so a connection returned to the pool can never leak a tenant context to the next request. (`SET LOCAL` itself cannot take bind parameters, hence `set_config`.)

Two supporting pieces:

- `tenancy.VerifyPooledSafety` (startup, pooled mode only) — refuses to boot if the connected role is a superuser or has `BYPASSRLS`, either of which silently ignores all policies. `docker/postgres/init.sql` creates the `bubblepulse_app` role with `NOSUPERUSER NOBYPASSRLS` for exactly this reason.
- `tenancy.IsValidUUID` — canonical-format validation applied to every UUID that arrives from a request path or body before it reaches SQL.

## Repository discipline: the `Querier` convention

`repository.Querier` (`internal/db/repository/querier.go`) is the pgx subset shared by `*pgxpool.Pool` and `pgx.Tx`. The convention:

- Methods on **RLS tables** take a `Querier` parameter and must be called with a transaction opened by `RunTx`. Their SQL deliberately has **no `org_id` filter** — RLS is the only scoping (e.g. `UserRepo.ListByOrg` is `SELECT ... FROM users` with no WHERE).
- Methods on **Global Directory tables** use the repo's own pool directly and take no `Querier` (e.g. `SessionRepo.FindByToken`, `OrgRepo.RenameOrg`).
- Inserts into RLS tables pass `org_id` explicitly (`UpsertUser`, `CreateTeam`, `InsertTx`, `InsertTopics`) so the policy's `WITH CHECK` can verify it against the bound tenant.

> ⚠️ This is a **convention, not compiler-enforced**: the type system would allow passing the pool where a tenant-bound `Querier` is expected. The integration suite (`internal/db/repository/*_integration_test.go`) exists to catch violations behaviorally — see [testing.md](testing.md).

## Tenant-context propagation — the three entry paths

Every path that touches tenant data establishes the context the same way: resolve org via the Global Directory, then `tenancy.WithTenantID(ctx, orgID)` → `RunTx`.

1. **HTTP request** — `requireSession` (`internal/api/middleware.go`) looks up the `session` cookie in `sessions` (Global Directory), injects the user ID, and calls `WithTenantID` with the session's `org_id`. In pooled mode a session **without** an org (created pre-tenancy) is rejected with 401, forcing re-authentication and provisioning. In siloed mode it is allowed through.
2. **Slack webhook** — `MessageService.Handle` resolves `user_identities` → org (falling back to `platform_workspaces` by workspace ID, backfilling the identity), then runs the insert + job enqueue inside `RunTx(WithTenantID(ctx, orgID), …)`.
3. **Background job** — the org ID is serialized into the River job (`jobs.NLPProcessingArgs.OrgID`) at enqueue time; `NLPWorker.Work` re-establishes the context from the args. Jobs with an empty org ID are cancelled (`river.JobCancel`), not retried.

## Org resolution & provisioning (OIDC callback)

`auth.Handler.resolveOrg` maps a verified login to an org in order of preference:

1. Existing identity with a stored org (`user_identities`).
2. The provider's workspace claim → `platform_workspaces` lookup.
3. **Auto-provisioning**: create an org (named from the workspace claim when inferable) and claim the workspace mapping in one Global Directory transaction. A concurrent first login from the same workspace is handled by `ClaimWorkspace`'s `INSERT … ON CONFLICT DO NOTHING` + winner re-read: the loser rolls back its candidate org and joins the winner's.

The first user of a freshly provisioned org becomes `ADMIN`; everyone who joins an existing org starts as `UPDATER`.

## Role matrix

| Capability | ADMIN | TEAM_EDITOR | UPDATER |
|---|---|---|---|
| Post updates (Slack DM), view dashboard, list teams | ✔ | ✔ | ✔ |
| List org users (`GET /api/v1/users`) | ✔ | ✔ | — |
| Rename **own** team | ✔ | ✔ | — |
| Create / delete teams, rename any team | ✔ | — | — |
| Move **unassigned** users into own team / remove own-team members | ✔ | ✔ | — |
| Assign any user to any team | ✔ | — | — |
| Change roles | ✔ (cannot demote the **last** admin → 409) | — | — |
| Rename the organization | ✔ | — | — |

Roles are re-read from the database on **every** guarded request (`requireRole`), so changes take effect immediately without session invalidation.

A second line of defense in `handleUpdateUser`: Postgres foreign-key validation bypasses RLS, so before assigning a team the handler calls `FindTeamByID` inside the same tenant transaction — a team UUID belonging to another org is invisible there and yields 404, never a cross-tenant link.

## Invariants & known sharp edges

Documented so they are deliberate, tested, or fixed — not rediscovered:

1. **`org_id` is nullable on all RLS tables** (migrations 011–015). A write that bypassed `RunTx` could create an orphan row invisible in pooled mode. Combined with the convention-only `Querier` discipline, this is the main structural risk; consider `NOT NULL` once legacy rows are migrated.
2. **Cross-tenant mutations fail silently.** `SetTeam`/`SetRole`/`RenameTeam`/`DeleteTeam` against another org's ID hit zero rows under RLS and surface as not-found. Safe, but quiet — the integration suite pins this behavior.
3. **`GET /api/dashboard?team_id=` accepts any UUID** from any authenticated user. A foreign org's team ID yields empty results (RLS on the joined tables), not an error; there is no check that the team belongs to the caller's org, nor that an UPDATER queries only their own team.
4. **`RenameOrg` runs on the Global Directory with no DB-layer tenant defense** — the org ID comes from the session-validated user record, but a bug that supplied the wrong ID would rename another org.
5. **Siloed mode accepts legacy sessions without an org** (the 401 rejection is pooled-only) and runs every query with the bypass GUC — acceptable for a true single-tenant deployment, wrong if a siloed DB ever holds two orgs.
6. **`handleListTeams` requires only a session, not a role** — every member, including UPDATERs, can enumerate their org's team names/IDs (needed for the dashboard scope selector). Information disclosure by design.
7. **`UpsertUser` in the OIDC callback receives the original (tenant-less) `ctx`** while the transaction was opened with the enriched one. Correct today because the repo only uses its `Querier`, but any future `TenantIDFromContext(ctx)` inside that call would find nothing.
