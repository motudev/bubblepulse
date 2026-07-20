# Testing

The test suite follows the ISTQB test-level split defined in `CLAUDE.md`, using only the standard `testing` package (no assertion libraries). Test design applies ISTQB techniques explicitly — equivalence partitioning, boundary value analysis, decision tables, state transitions — and **every suite covers negative paths**: expected failures assert the exact contract (status code, sentinel error, `RowsAffected == 0`), never just "no error".

| Level | Location | Build tag | Dependencies | Run with |
|---|---|---|---|---|
| Unit / component | co-located `*_test.go` | none | none (test doubles) | `make test` |
| Integration | `internal/db/**` `*_integration_test.go` | `//go:build integration` | Postgres via `TEST_DATABASE_URL` | `make test-integration` |
| System | `test/system/` | `//go:build system` | full stack | `make test-system` (no tests yet) |

## Unit tests

**`internal/tenancy`** — pure-function tests for context propagation (`WithTenantID` / `TenantIDFromContext`) and `IsValidUUID` (boundary lengths 35/36/37, non-hex, separator positions, injection strings).

**`internal/api`** — handler tests with `net/http/httptest` and hand-written fakes of the consumer interfaces the server already defines (`SessionLookup`, `UserStore`, `TeamStore`, `OrgStore`, `tenantTxRunner`, `dashboardQuerier`) in `fakes_test.go`. The fake runner invokes the callback with a nil `pgx.Tx` — the fakes never touch it. Covered contracts include:

- `requireSession` as a state-transition table: no cookie → 401, unknown token → 401, valid session without org → 401 pooled / pass siloed, valid session with org → tenant in context.
- `requireRole`: allow/deny per role partition, DB failure → 401.
- Team CRUD: role guards, TEAM_EDITOR own-team rule, UUID validation, not-found mapping.
- User management: the TEAM_EDITOR team-move **decision table** (only "remove own member" and "add unassigned to own team" pass), role-change guard, invalid-role partition, cross-tenant team assignment → 404, last-admin demotion boundary (1 admin → 409, 2 → 200).
- Org rename: 403 partitions, empty-name 400, and that the renamed org is always the actor's own.
- Dashboard: `team_id` validation, `[]`-not-`null` serialization, matrix diagonal/symmetry.

## Integration tests — tenant isolation

These are the acceptance bar for the multi-tenancy layer: they run the **real** RLS policies against the **real** migrations, connecting as the non-`BYPASSRLS` role.

**Setup** (`internal/db/testhelpers`):

- `testhelpers.Setup(t)` reads `TEST_DATABASE_URL` and **skips** (not fails) when unset, so the suite stays green without a database; it returns an `Env` bundling the pool, a pooled-mode `tenancy.Runner`, and the repositories.
- Migrations are applied programmatically with goose on first connect; app tables are truncated between tests.
- Seed helpers (`CreateOrg`, `CreateTeam`, `CreateUser`, …) insert through `tenancy.Runner.RunTx`, exercising the production write path including `WITH CHECK`.
- The connected role is verified non-superuser/non-`BYPASSRLS` (`tenancy.VerifyPooledSafety`) before any test runs — a bypassing role would make every isolation test pass vacuously.

**What is pinned** (`internal/db/repository/*_integration_test.go`), always with two seeded orgs A and B:

- Reads under A's tenant context return only A's rows; B's IDs are not found.
- Cross-tenant writes (`SetTeam`, `SetRole`, `RenameTeam`, `DeleteTeam`) are silent no-ops — B's rows are provably unmodified afterwards.
- Inserting into another tenant violates the policy's `WITH CHECK` and errors.
- **Fail-closed**: with no tenant GUC bound, every RLS table appears empty.
- **Siloed mode** sees all rows; **pooled mode without a tenant in context** returns `tenancy.ErrNoTenant` before touching the DB.
- The tenant GUC is transaction-local: after `RunTx` returns, pooled connections carry no tenant setting.
- Dashboard queries never leak org B's updates/topics into org A's results — even when A passes B's `team_id` as the filter.
- Global Directory semantics: `ClaimWorkspace` conflict handling, per-org email uniqueness (same email in two orgs = two users), identity org backfill.

## Running

```bash
make db-up          # starts Postgres; docker/postgres/init.sql provisions the
                    # bubblepulse_test database and the non-BYPASSRLS bubblepulse_app role
make test           # unit tests (Go, -race, coverage) + frontend vitest
make test-integration
```

`TEST_DATABASE_URL` comes from `.env` (see `.env.example`). Docker Compose maps the container's 5432 to host port **5001**:

```
TEST_DATABASE_URL=postgres://bubblepulse_app:bubblepulse_app@localhost:5001/bubblepulse_test?sslmode=disable
```

> The integration helpers refuse superuser/`BYPASSRLS` roles. If you point `TEST_DATABASE_URL` at the `postgres` user, the suite fails fast with `tenancy.ErrRLSBypassRole` instead of silently testing nothing.

## Frontend

Vitest + `@vue/test-utils` + jsdom, configured inline in `frontend/vite.config.ts`. Run with `npm run test` (or `npm run test:watch`). Current suites: `BubbleMap.test.ts` (component smoke) and `stores/scope.test.ts` (dashboard scope logic incl. rejection paths).

## Writing new tests

- Table-driven with `t.Run(tc.name, …)`; case names state the expected outcome (`team_editor_renaming_foreign_team_returns_403`).
- Keep roughly half of each table as expected-failure cases.
- Anything touching an RLS table belongs in the integration tier; unit tests must not open network or DB connections.
- New RLS tables must be added to the truncation list in `testhelpers` and get their own isolation cases (visibility, cross-tenant write no-op, fail-closed, WITH CHECK).
