# HTTP API

All application endpoints are JSON over HTTP, registered in `internal/api/server.go` using Go 1.22 method+path routing. Authentication is an opaque `session` cookie (HTTP-only, SameSite=Lax, 30-day expiry) issued by the OIDC callback.

Auth levels:

- **public** — no checks.
- **session** — `requireSession`: valid session cookie; in pooled mode the session must carry an org (401 otherwise).
- **ADMIN** / **ADMIN or TEAM_EDITOR** — `requireRole`: session + the user's role, re-read from the DB on every request (403 on mismatch).
- **Slack signature** — HMAC-SHA256 over `v0:<timestamp>:<body>` compared constant-time against `X-Slack-Signature`. The signing secret is per-app (same for all workspace installs).
- **Slack install state** — short-lived `slack_install_state` HTTP-only cookie compared against the `?state=` query parameter (CSRF guard for the OAuth install callback).

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/health` | public | Liveness probe |
| GET | `/api/v1/config` | public | App capability flags for the frontend (e.g. which OIDC provider is active) |
| GET | `/api/auth/login` | public | Redirects to the OIDC authorization endpoint (sets `oidc_state`/`oidc_nonce` cookies) |
| GET | `/api/auth/callback` | public | Code exchange, org resolution/provisioning, user upsert, session issuance; redirects to `/dashboard` |
| GET | `/api/auth/logout` | public | Deletes the session, clears the cookie, RP-initiated logout when the provider supports it |
| GET | `/api/v1/me` | session | Current user profile |
| GET | `/api/dashboard` | session | Bubble-map payload; optional `?team_id=<uuid>` |
| GET | `/api/v1/teams` | session | List the org's teams |
| POST | `/api/v1/teams` | ADMIN | Create a team |
| PATCH | `/api/v1/teams/{id}` | ADMIN or TEAM_EDITOR | Rename a team (TEAM_EDITOR: own team only) |
| DELETE | `/api/v1/teams/{id}` | ADMIN | Delete a team (members fall back to unassigned) |
| GET | `/api/v1/users` | ADMIN or TEAM_EDITOR | List the org's users |
| PATCH | `/api/v1/users/{id}` | ADMIN or TEAM_EDITOR | Change a user's team and/or role |
| PATCH | `/api/v1/org` | ADMIN | Rename the caller's organization |
| POST | `/api/slack/events` | Slack signature | Slack Events API webhook |
| GET | `/api/slack/install` | public | Redirects to the Slack OAuth v2 authorization URL (sets `slack_install_state` cookie); only registered when `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` are configured |
| GET | `/api/slack/callback` | Slack install state | Exchanges the OAuth code for a per-workspace bot token, provisions the org if new, stores `bot_token` in `platform_workspaces`, redirects to the frontend |

RLS scopes every list/read to the caller's organization automatically — none of the handlers filter by org in SQL. See [multi-tenancy.md](multi-tenancy.md).

## Shapes

### `GET /api/v1/health` → 200

```json
{"status": "ok"}
```

### `GET /api/v1/config` → 200

Public — no auth required. Returns feature flags the SPA needs before login.

```json
{"slack_oidc": true}
```

- `slack_oidc` — `true` when `OIDC_ISSUER_URL` is `https://slack.com`; controls whether the login page shows the "Sign in with Slack" branded button or a generic "Sign in" button.

### `GET /api/v1/me` → 200

```json
{
  "id": 1,
  "email": "alice@example.com",
  "name": "Alice",
  "role": "ADMIN",
  "team_id": "b7c9…-uuid-or-null",
  "org": {"id": "a1b2…-uuid", "name": "Acme"},
  "slack_install_enabled": true
}
```

`org` is `null` only for legacy users without an organization (siloed mode). `slack_install_enabled` is `true` when `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` are configured on the backend — the frontend uses this to decide whether to show the "Add to Slack" install modal.

### `GET /api/dashboard[?team_id=<uuid>]` → 200

`team_id` must be a canonical UUID (400 otherwise). It restricts results to that team's members; org scoping always applies on top via RLS — a foreign org's team ID yields empty results, not an error.

```json
{
  "users": [
    {
      "id": 1,
      "name": "Alice",
      "email": "alice@example.com",
      "update_text": "Shipped the auth PR",
      "update_at": "2026-07-13T09:00:00Z",
      "topics": ["ship auth pr"]
    }
  ],
  "topics": ["deploy pipeline", "ship auth pr"],
  "similarity_matrix": [[1.0, 0.42], [0.42, 1.0]]
}
```

- `users` includes **every** visible user; `update_text`/`update_at` are `null` when they haven't posted today. `topics` per user is never `null`.
- `topics` (top level) is the sorted, deduplicated union of all users' topics for today.
- `similarity_matrix` is N×N (N = `len(topics)`), symmetric, diagonal `1.0`; off-diagonals are pairwise cosine similarities of the topic embeddings (missing pairs default to `0.0`). Arrays serialize as `[]`, never `null`.

### Teams

`GET /api/v1/teams` → 200 `[{"id": "<uuid>", "name": "Platform"}, …]`

`POST /api/v1/teams` body `{"name": "Platform"}` → 201 `{"id": "<uuid>", "name": "Platform"}`
- 400 if the body is malformed or `name` is empty/whitespace (the name is trimmed).

`PATCH /api/v1/teams/{id}` body `{"name": "New name"}` → 200 `{"id", "name"}`
- 400 invalid UUID or empty name · 403 TEAM_EDITOR renaming a team that is not their own · 404 team not visible in this org.

`DELETE /api/v1/teams/{id}` → 204
- 400 invalid UUID · 404 not visible in this org.

### Users

`GET /api/v1/users` → 200

```json
[{"id": 1, "name": "Alice", "email": "alice@example.com", "role": "ADMIN", "team_id": null}]
```

`PATCH /api/v1/users/{id}` — partial update; absent fields are unchanged, `"team_id": null` explicitly unassigns:

```json
{"team_id": "<uuid>|null", "role": "ADMIN|TEAM_EDITOR|UPDATER"}
```

Responses:
- 200 — updated user entry (same shape as the list).
- 400 — non-numeric ID, malformed body, both fields absent, invalid role value, or malformed team UUID.
- 403 — TEAM_EDITOR attempting a role change; or a team change outside the two permitted TEAM_EDITOR moves (remove a member of their own team; add an **unassigned** user to their own team).
- 404 — target user not visible in this org, or `team_id` not visible in this org (cross-tenant defense).
- 409 — demoting the organization's **last** ADMIN.

### `PATCH /api/v1/org` body `{"name": "Acme"}` → 200 `{"id", "name"}`

- 400 empty name · 404 org not found. The org renamed is always the caller's own (taken from the authenticated user record, never the request).

### `POST /api/slack/events`

- 401 on signature failure. `url_verification` envelopes are answered with the plaintext challenge. `event_callback` is acked with 200 **before** processing (Slack retries non-2xx); only DM messages (`channel_type == "im"`) from non-bot users with text are ingested. Unknown senders / unmapped workspaces are logged and dropped.

### `GET /api/slack/install`

Only registered when `SLACK_CLIENT_ID` and `SLACK_CLIENT_SECRET` are present. Sets a `slack_install_state` cookie (HttpOnly, SameSite=Lax, 10-minute TTL, path scoped to `/api/slack/callback`) then redirects to `https://slack.com/oauth/v2/authorize` with `client_id`, `scope`, `redirect_uri`, and `state`.

### `GET /api/slack/callback`

- 400 on missing or mismatched `state` vs. the `slack_install_state` cookie, or missing `code`.
- 502 if the `oauth.v2.access` call to Slack fails or returns `ok: false`.
- 500 on database errors (org provisioning or token storage).
- On success: provisions or joins the org (race-safe — concurrent installs from the same workspace converge to one org), stores the workspace bot token in `platform_workspaces`, and redirects to `/dashboard?slack_installed=1`.

## Error format

Errors are plain-text bodies via `http.Error` (e.g. `forbidden`, `team not found`, `cannot demote the last admin`) with the status codes above. Unexpected failures return 500 `internal server error` and are logged with `slog`.
