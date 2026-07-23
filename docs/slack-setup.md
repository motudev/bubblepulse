# Slack Setup Guide

BubblePulse uses two independent Slack integrations that must each be configured separately in the [Slack API console](https://api.slack.com/apps):

| Integration | Purpose | Required env var |
|---|---|---|
| **OIDC (Sign in with Slack)** | User login — maps each person's Slack user ID to their BubblePulse account | `OIDC_*` |
| **Bot (Events API + OAuth v2)** | Receives daily standup DMs from team members | `SLACK_*` |

> **Important:** Both integrations require the same Slack app. Create one app, configure it for both purposes or use the OfficialBubblePulse app(if your not self hosting)

---

## Step 1 — Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and click **Create New App → From scratch**.
2. Give it a name (e.g. "BubblePulse") and pick your workspace.
3. Keep the app settings page open — you will copy values from it in the steps below.

---

## Step 2 — Configure Sign in with Slack (OIDC)

This lets team members authenticate to BubblePulse with their Slack credentials. It is also what links each Slack user ID to their BubblePulse account — a prerequisite for DM message ingestion.

### 2a — Add OIDC scopes

Under **OAuth & Permissions → Scopes → User Token Scopes**, add:

| Scope | Why |
|---|---|
| `openid` | Required for OIDC |
| `email` | Populates `users.email` |
| `profile` | Populates `users.name` |

### 2b — Add the OIDC redirect URL

Under **OAuth & Permissions → Redirect URLs**, add:

```
https://your-domain/api/auth/callback
```

### 2c — Copy OIDC credentials

Under **Basic Information → App Credentials**, copy:

| Credential | Env var |
|---|---|
| Client ID | `OIDC_CLIENT_ID` |
| Client Secret | `OIDC_CLIENT_SECRET` |

Set these in your `.env` / server environment:

```bash
OIDC_ISSUER_URL=https://slack.com
OIDC_CLIENT_ID=<your-client-id>
OIDC_CLIENT_SECRET=<your-client-secret>
OIDC_REDIRECT_URL=https://your-domain/api/auth/callback
```

---

## Step 3 — Configure the Bot (DM ingestion + Events API)

This allows team members to DM the bot their daily update, which BubblePulse receives and processes.

### 3a — Add bot token scopes

Under **OAuth & Permissions → Scopes → Bot Token Scopes**, add:

| Scope | Why |
|---|---|
| `im:history` | Receive DMs sent to the bot |
| `chat:write` | Send messages back (future outbound use) |

### 3b — Add the OAuth install redirect URL

Under **OAuth & Permissions → Redirect URLs**, add (alongside the OIDC redirect URL from step 2b):

```
https://your-domain/api/slack/callback
```

### 3c — Enable the Events API

Under **Event Subscriptions**:

1. Toggle **Enable Events** ON.
2. Set the **Request URL** to:
   ```
   https://your-domain/api/slack/events
   ```
   Slack will immediately send a `url_verification` challenge. The backend must be running and `SLACK_SIGNING_SECRET` must be set for this to pass (see step 3d).

3. Under **Subscribe to bot events**, add:

   | Event | Why |
   |---|---|
   | `message.im` | Fires when a user sends the bot a DM |

4. Click **Save Changes**.

### 3d — Copy bot credentials

Under **Basic Information → App Credentials**, copy the **Signing Secret** (not the Client Secret — it is a different field):

| Credential | Env var |
|---|---|
| Signing Secret | `SLACK_SIGNING_SECRET` |
| Client ID | `SLACK_CLIENT_ID` |
| Client Secret | `SLACK_CLIENT_SECRET` |

```bash
SLACK_SIGNING_SECRET=<your-signing-secret>
SLACK_CLIENT_ID=<your-client-id>      # same value as OIDC_CLIENT_ID when using one app
SLACK_CLIENT_SECRET=<your-client-secret>
SLACK_INSTALL_REDIRECT_URL=https://your-domain/api/slack/callback
```

> `SLACK_SIGNING_SECRET` is the per-app HMAC key used to verify every inbound webhook. Without it the `POST /api/slack/events` route is never registered and no DMs will be received.

---

## Step 4 — Install the App to Your Workspace

After configuring bot scopes and Event Subscriptions, install (or re-install) the app:

- Under **Install App**, click **Install to Workspace** and approve the permissions.

Or use BubblePulse's own install flow (once the backend is running):

1. Log in to BubblePulse as an ADMIN.
2. Click **Add to Slack** on the dashboard or in **Admin Settings → Slack Integration**.
3. Approve the permissions on the Slack authorization page.
4. You are redirected to `/dashboard?slack_installed=1`.

---

## Step 5 — Team Members Send Their First DM

Each team member must:

1. Open Slack and find the BubblePulse bot (search for the app name in the sidebar).
2. Send it a DM with their daily update, e.g.:
   ```
   Finished the auth refactor. Working on the data-model migration today.
   ```

The message appears in BubblePulse's dashboard after the NLP pipeline processes it (usually within a few seconds).

> **Why must users log in with Slack first?** BubblePulse maps an incoming DM to the correct user account using the Slack user ID (`U…`), which is stored in the database when the user first logs in via Sign in with Slack. If a team member DMs the bot before their first login, the message is logged as `platform user not registered` and discarded. Once they log in, future DMs are accepted.

---

## Complete Environment Variable Reference

```bash
# ── OIDC (user login) ──────────────────────────────────────────────────────
OIDC_ISSUER_URL=https://slack.com
OIDC_CLIENT_ID=<client-id>
OIDC_CLIENT_SECRET=<client-secret>
OIDC_REDIRECT_URL=https://your-domain/api/auth/callback

# ── Slack bot (DM ingestion + OAuth install flow) ──────────────────────────
SLACK_SIGNING_SECRET=<signing-secret>        # Basic Information → App Credentials
SLACK_CLIENT_ID=<client-id>                  # same value as OIDC_CLIENT_ID
SLACK_CLIENT_SECRET=<client-secret>          # same value as OIDC_CLIENT_SECRET
SLACK_INSTALL_REDIRECT_URL=https://your-domain/api/slack/callback

# ── Siloed-mode only (single tenant, no OAuth install flow) ────────────────
# SLACK_BOT_TOKEN=xoxb-...                   # paste the xoxb token directly; no install flow needed
```

---

## Troubleshooting

### Messages sent to the bot do not appear in BubblePulse

Work through these checks in order:

| Check | How to verify |
|---|---|
| `SLACK_SIGNING_SECRET` is set | Backend logs at startup print a warning if it is missing. Check `docker logs` or your server log. |
| Event Subscriptions → Request URL is saved and verified | Open the Slack app settings. The URL field shows a green tick when verified. |
| `message.im` is subscribed under bot events | Open Event Subscriptions → Subscribe to bot events. |
| The app is installed to the workspace | Under Install App, the button should say "Reinstall to Workspace". If it says "Install", the app is not yet installed. |
| The user has logged in to BubblePulse at least once | If the backend logs `platform user not registered` for the Slack user ID, the identity link does not exist yet. Ask the user to sign in first. |
| The user is DMing the correct bot | They must DM the BubblePulse app, not a person. The app appears under **Apps** in the Slack sidebar. |

### `redirect_uri did not match any configured URIs`

The redirect URI passed to Slack during the OAuth flow does not match any URL listed in **OAuth & Permissions → Redirect URLs**. Add the exact URI the backend sends (check `SLACK_INSTALL_REDIRECT_URL` and `OIDC_REDIRECT_URL`) to the Slack app's Redirect URLs list.

### `url_verification` challenge fails when saving the Event Subscriptions Request URL

The backend is not reachable from Slack's servers, or `SLACK_SIGNING_SECRET` is not set. Ensure:
- The backend is deployed and accessible at the URL you entered.
- `SLACK_SIGNING_SECRET` is set in the environment and the backend has been restarted since it was added.
- There is no firewall or reverse-proxy rule blocking `POST /api/slack/events`.
