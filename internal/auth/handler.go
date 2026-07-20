package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// UserRepository persists users (RLS-protected) and their external
// identities (Global Directory).
type UserRepository interface {
	UpsertUser(ctx context.Context, q repository.Querier, orgID, email, name, roleIfNew string) (int64, error)
	UpsertIdentity(ctx context.Context, userID int64, provider, providerID, orgID string) error
	FindIdentity(ctx context.Context, provider, providerID string) (repository.IdentityRecord, error)
}

// SessionRepository persists opaque session tokens.
type SessionRepository interface {
	Create(ctx context.Context, userID int64, token, orgID string) error
	Delete(ctx context.Context, token string) error
}

// OrgRepository creates organizations during auto-provisioning.
type OrgRepository interface {
	CreateOrg(ctx context.Context, q repository.Querier, name string) (string, error)
}

// WorkspaceRepository maps provider workspace/tenant IDs to organizations.
type WorkspaceRepository interface {
	FindOrgByWorkspace(ctx context.Context, provider, externalID string) (string, error)
	ClaimWorkspace(ctx context.Context, q repository.Querier, orgID, provider, externalID string) (string, bool, error)
}

// tenantTxRunner opens tenant-scoped transactions (satisfied by *tenancy.Runner).
type tenantTxRunner interface {
	RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Config carries the OIDC fields needed to configure the Handler.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string // origin of the SPA (e.g. http://localhost:5200); empty = same origin as backend
}

// Handler holds the OIDC provider and wires the login/callback/logout HTTP handlers.
type Handler struct {
	provider              *oidc.Provider
	oauth2Cfg             oauth2.Config
	verifier              *oidc.IDTokenVerifier
	issuerURL             string
	secure                bool
	frontendURL           string // SPA origin; empty = same origin as backend
	endSessionEndpoint    string // empty if provider doesn't advertise one
	postLogoutRedirectURI string // where to land after provider logout
	pool                  *pgxpool.Pool // Global Directory transactions (org provisioning)
	runner                tenantTxRunner
	users                 UserRepository
	sessions              SessionRepository
	orgs                  OrgRepository
	workspaces            WorkspaceRepository
}

// Repos bundles the persistence dependencies of the Handler.
type Repos struct {
	Users      UserRepository
	Sessions   SessionRepository
	Orgs       OrgRepository
	Workspaces WorkspaceRepository
}

// NewHandler constructs a Handler from a discovered OIDC provider.
func NewHandler(provider *oidc.Provider, cfg Config, pool *pgxpool.Pool, runner tenantTxRunner, repos Repos) *Handler {
	h := &Handler{
		provider: provider,
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier:    provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		issuerURL:   cfg.IssuerURL,
		secure:      strings.HasPrefix(cfg.RedirectURL, "https://"),
		frontendURL: cfg.FrontendURL,
		pool:        pool,
		runner:      runner,
		users:       repos.Users,
		sessions:    repos.Sessions,
		orgs:        repos.Orgs,
		workspaces:  repos.Workspaces,
	}

	// Extract end_session_endpoint from the OIDC discovery document if present.
	var meta struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	_ = provider.Claims(&meta)
	h.endSessionEndpoint = meta.EndSessionEndpoint

	// Post-logout redirect URI: prefer FRONTEND_URL, fall back to backend origin.
	if cfg.FrontendURL != "" {
		h.postLogoutRedirectURI = strings.TrimRight(cfg.FrontendURL, "/") + "/"
	} else if u, err := url.Parse(cfg.RedirectURL); err == nil {
		h.postLogoutRedirectURI = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	}

	return h
}

// Login generates a CSRF state token and a replay-prevention nonce, stores both in
// short-lived HTTP-only cookies, and redirects the user to the OIDC authorization endpoint.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shortLived := &http.Cookie{
		Path:     "/api/auth/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	}
	stateCookie := *shortLived
	stateCookie.Name = "oidc_state"
	stateCookie.Value = state
	http.SetCookie(w, &stateCookie)

	nonceCookie := *shortLived
	nonceCookie.Name = "oidc_nonce"
	nonceCookie.Value = nonce
	http.SetCookie(w, &nonceCookie)

	authURL := h.oauth2Cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback verifies the CSRF state and nonce, exchanges the authorization code for tokens,
// verifies the id_token, upserts the user and identity, creates a session, and
// redirects the browser to the dashboard.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || stateCookie.Value != r.FormValue("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	nonceCookie, err := r.Cookie("oidc_nonce")
	if err != nil {
		http.Error(w, "missing nonce", http.StatusBadRequest)
		return
	}
	clear := &http.Cookie{
		Path:     "/api/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	}
	clearState := *clear
	clearState.Name = "oidc_state"
	http.SetCookie(w, &clearState)
	clearNonce := *clear
	clearNonce.Name = "oidc_nonce"
	http.SetCookie(w, &clearNonce)

	oauth2Token, err := h.oauth2Cfg.Exchange(ctx, r.FormValue("code"))
	if err != nil {
		slog.Error("oauth2 token exchange failed", "error", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing id_token", http.StatusInternalServerError)
		return
	}

	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		slog.Error("id_token verification failed", "error", err)
		http.Error(w, "token verification failed", http.StatusUnauthorized)
		return
	}

	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		slog.Error("claims extraction failed", "error", err)
		http.Error(w, "claims extraction failed", http.StatusInternalServerError)
		return
	}
	if nonce, _ := rawClaims["nonce"].(string); nonce != nonceCookie.Value {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	claims := normalizeClaims(h.issuerURL, rawClaims)

	orgID, isNewOrg, err := h.resolveOrg(ctx, claims)
	if err != nil {
		slog.Error("organization resolution failed", "error", err)
		http.Error(w, "organization resolution failed", http.StatusInternalServerError)
		return
	}

	// The creator of a freshly provisioned organization becomes its first ADMIN;
	// everyone joining an existing organization starts as UPDATER.
	roleIfNew := repository.RoleUpdater
	if isNewOrg {
		roleIfNew = repository.RoleAdmin
	}

	// users is RLS-protected: the upsert must run inside a tenant-scoped
	// transaction so the WITH CHECK clause accepts the row.
	var userID int64
	err = h.runner.RunTx(tenancy.WithTenantID(ctx, orgID), func(tx pgx.Tx) error {
		var txErr error
		userID, txErr = h.users.UpsertUser(ctx, tx, orgID, claims.Email, claims.Name, roleIfNew)
		return txErr
	})
	if err != nil {
		slog.Error("upsert user failed", "error", err)
		http.Error(w, "user persistence failed", http.StatusInternalServerError)
		return
	}

	if err := h.users.UpsertIdentity(ctx, userID, h.issuerURL, claims.Sub, orgID); err != nil {
		slog.Error("upsert identity failed", "error", err)
		http.Error(w, "identity persistence failed", http.StatusInternalServerError)
		return
	}

	sessionToken, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.Create(ctx, userID, sessionToken, orgID); err != nil {
		slog.Error("session creation failed", "error", err)
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	const thirtyDays = 60 * 60 * 24 * 30
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   thirtyDays,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.frontendURL+"/dashboard", http.StatusFound)
}

// Logout deletes the server-side session, clears the session cookie, and redirects
// to the OIDC provider's end_session_endpoint (RP-initiated logout) when available,
// otherwise to the app root. Safe to call with a missing or expired cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		// Best-effort delete — ignore errors (token may already be expired/absent).
		_ = h.sessions.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})

	if h.endSessionEndpoint != "" {
		params := url.Values{}
		params.Set("post_logout_redirect_uri", h.postLogoutRedirectURI)
		http.Redirect(w, r, h.endSessionEndpoint+"?"+params.Encode(), http.StatusFound)
		return
	}
	http.Redirect(w, r, h.frontendURL+"/", http.StatusFound)
}

// resolveOrg maps a verified login to an organization using the Global
// Directory, in order of preference: the user's existing identity, the
// provider's workspace/tenant claim, and finally auto-provisioning a new
// organization. isNewOrg reports that this login created the organization.
func (h *Handler) resolveOrg(ctx context.Context, claims UserClaims) (orgID string, isNewOrg bool, err error) {
	ident, err := h.users.FindIdentity(ctx, h.issuerURL, claims.Sub)
	switch {
	case err == nil && ident.OrgID != nil:
		return *ident.OrgID, false, nil
	case err != nil && !errors.Is(err, repository.ErrIdentityNotFound):
		return "", false, fmt.Errorf("find identity: %w", err)
	}

	// Unknown identity (or a legacy one without an org): try the workspace claim.
	if claims.WorkspaceID != "" {
		orgID, err = h.workspaces.FindOrgByWorkspace(ctx, h.issuerURL, claims.WorkspaceID)
		if err == nil {
			return orgID, false, nil
		}
		if !errors.Is(err, repository.ErrWorkspaceNotFound) {
			return "", false, fmt.Errorf("find workspace: %w", err)
		}
	}

	return h.provisionOrg(ctx, claims)
}

// provisionOrg creates a new organization (name from the workspace claim when
// inferable, blank otherwise) and atomically claims the workspace mapping for
// it. If a concurrent first login from the same workspace wins the claim, the
// candidate organization is rolled back and the winner's is returned instead.
func (h *Handler) provisionOrg(ctx context.Context, claims UserClaims) (orgID string, isNewOrg bool, err error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("begin provisioning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	orgID, err = h.orgs.CreateOrg(ctx, tx, claims.WorkspaceName)
	if err != nil {
		return "", false, fmt.Errorf("create organization: %w", err)
	}

	if claims.WorkspaceID != "" {
		ownerOrgID, created, err := h.workspaces.ClaimWorkspace(ctx, tx, orgID, h.issuerURL, claims.WorkspaceID)
		if err != nil {
			return "", false, fmt.Errorf("claim workspace: %w", err)
		}
		if !created {
			// Lost a concurrent provisioning race: discard our candidate org.
			if err := tx.Rollback(ctx); err != nil {
				return "", false, fmt.Errorf("rollback candidate organization: %w", err)
			}
			return ownerOrgID, false, nil
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", false, fmt.Errorf("commit provisioning transaction: %w", err)
	}
	slog.Info("auto-provisioned organization", "org_id", orgID, "workspace", claims.WorkspaceID, "name", claims.WorkspaceName)
	return orgID, true, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
