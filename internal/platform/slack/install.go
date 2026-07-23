package slack

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/messaging"
)

const (
	slackOAuthEndpoint     = "https://slack.com/api/oauth.v2.access"
	slackAuthorizeURL      = "https://slack.com/oauth/v2/authorize"
	slackProvider          = "https://slack.com"
	installStateCookieName = "slack_install_state"
	// defaultBotScopes covers DM ingestion (im:history) and future outbound
	// messaging (chat:write). Extend as new features are added.
	defaultBotScopes = "im:history,chat:write"
)

// orgCreator creates a new organization record.
// Satisfied by *repository.OrgRepo.
type orgCreator interface {
	CreateOrg(ctx context.Context, q repository.Querier, name string) (string, error)
}

// workspaceProvisioner handles workspace-org mapping and bot token storage.
// Satisfied by *repository.WorkspaceRepo.
type workspaceProvisioner interface {
	ClaimWorkspace(ctx context.Context, q repository.Querier, orgID, provider, externalID string) (string, bool, error)
	UpsertBotToken(ctx context.Context, provider, externalID, teamName, botToken string) error
}

// Installer handles the Slack OAuth v2 app-installation flow, exposing two
// PlatformAdapter values (install redirect and OAuth callback) via Adapters().
type Installer struct {
	clientID     string
	clientSecret string
	redirectURL  string
	frontendURL  string
	secure       bool
	pool         *pgxpool.Pool
	orgs         orgCreator
	workspaces   workspaceProvisioner
}

// NewInstaller constructs an Installer. redirectURL must match the URI
// registered in the Slack app settings and point to GET /api/slack/callback.
func NewInstaller(
	clientID, clientSecret, redirectURL, frontendURL string,
	pool *pgxpool.Pool,
	orgs orgCreator,
	workspaces workspaceProvisioner,
) *Installer {
	return &Installer{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		frontendURL:  frontendURL,
		secure:       strings.HasPrefix(redirectURL, "https://"),
		pool:         pool,
		orgs:         orgs,
		workspaces:   workspaces,
	}
}

// Adapters returns the two PlatformAdapter values to register with the HTTP mux.
func (inst *Installer) Adapters() []messaging.PlatformAdapter {
	return []messaging.PlatformAdapter{
		&installRedirectAdapter{inst},
		&installCallbackAdapter{inst},
	}
}

// installRedirectAdapter handles GET /api/slack/install.
type installRedirectAdapter struct{ inst *Installer }

func (a *installRedirectAdapter) Route() string              { return "GET /api/slack/install" }
func (a *installRedirectAdapter) Handler() http.HandlerFunc { return a.inst.handleInstall }

// installCallbackAdapter handles GET /api/slack/callback.
type installCallbackAdapter struct{ inst *Installer }

func (a *installCallbackAdapter) Route() string              { return "GET /api/slack/callback" }
func (a *installCallbackAdapter) Handler() http.HandlerFunc { return a.inst.handleCallback }

func (inst *Installer) handleInstall(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(32)
	if err != nil {
		slog.Error("slack install: failed to generate state token", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     installStateCookieName,
		Value:    state,
		Path:     "/api/slack/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   inst.secure,
		SameSite: http.SameSiteLaxMode,
	})

	params := url.Values{
		"client_id":    {inst.clientID},
		"scope":        {defaultBotScopes},
		"redirect_uri": {inst.redirectURL},
		"state":        {state},
	}
	http.Redirect(w, r, slackAuthorizeURL+"?"+params.Encode(), http.StatusFound)
}

func (inst *Installer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify CSRF state before doing anything else.
	cookie, err := r.Cookie(installStateCookieName)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Clear the state cookie immediately.
	http.SetCookie(w, &http.Cookie{
		Name:     installStateCookieName,
		Path:     "/api/slack/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   inst.secure,
		SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for a workspace bot token.
	oauthResp, err := inst.exchangeCode(r.Context(), code)
	if err != nil {
		slog.Error("slack install: OAuth code exchange failed", "error", err)
		http.Error(w, "OAuth exchange failed", http.StatusBadGateway)
		return
	}

	// Provision (or join) the org for this workspace.
	// Organizations and platform_workspaces are both Global Directory tables
	// (no RLS), so a plain pgx transaction is sufficient.
	ownerOrgID, err := inst.provisionWorkspace(r.Context(), oauthResp)
	if err != nil {
		slog.Error("slack install: workspace provisioning failed",
			"team_id", oauthResp.Team.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Store (or refresh) the bot token outside the provisioning transaction —
	// UpsertBotToken is idempotent and the workspace row is guaranteed to exist.
	if err := inst.workspaces.UpsertBotToken(
		r.Context(),
		slackProvider,
		oauthResp.Team.ID,
		oauthResp.Team.Name,
		oauthResp.AccessToken,
	); err != nil {
		slog.Error("slack install: failed to store bot token",
			"team_id", oauthResp.Team.ID, "org_id", ownerOrgID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("slack install: workspace installed successfully",
		"team_id", oauthResp.Team.ID,
		"team_name", oauthResp.Team.Name,
		"org_id", ownerOrgID,
	)

	http.Redirect(w, r, inst.frontendURL+"/dashboard?slack_installed=1", http.StatusFound)
}

// provisionWorkspace atomically creates a candidate org and claims the
// workspace. If the workspace was already claimed by another org, the
// candidate org creation is rolled back and the existing owner's ID is returned.
func (inst *Installer) provisionWorkspace(ctx context.Context, resp *oauthResponse) (string, error) {
	tx, err := inst.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	candidateOrgID, err := inst.orgs.CreateOrg(ctx, tx, resp.Team.Name)
	if err != nil {
		return "", fmt.Errorf("create org: %w", err)
	}

	ownerOrgID, created, err := inst.workspaces.ClaimWorkspace(
		ctx, tx, candidateOrgID, slackProvider, resp.Team.ID,
	)
	if err != nil {
		return "", fmt.Errorf("claim workspace: %w", err)
	}

	if !created {
		// Another org already owns this workspace; rollback the candidate.
		_ = tx.Rollback(ctx)
		return ownerOrgID, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return ownerOrgID, nil
}

// oauthResponse is the relevant subset of the oauth.v2.access JSON response.
type oauthResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID string `json:"id"`
	} `json:"authed_user"`
}

// exchangeCode posts the authorization code to Slack's oauth.v2.access
// endpoint and returns the parsed response.
func (inst *Installer) exchangeCode(ctx context.Context, code string) (*oauthResponse, error) {
	form := url.Values{
		"code":         {code},
		"redirect_uri": {inst.redirectURL},
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		slackOAuthEndpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(inst.clientID, inst.clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result oauthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode oauth response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("slack API error: %s", result.Error)
	}
	if result.AccessToken == "" || result.Team.ID == "" {
		return nil, fmt.Errorf("slack API returned empty access_token or team.id")
	}
	return &result, nil
}

// randomHex generates a cryptographically random hex string of length 2*n.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
