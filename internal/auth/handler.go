package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// UserRepository persists users and their external identities.
type UserRepository interface {
	UpsertUser(ctx context.Context, email, name string) (int64, error)
	UpsertIdentity(ctx context.Context, userID int64, provider, providerID string) error
}

// SessionRepository persists opaque session tokens.
type SessionRepository interface {
	Create(ctx context.Context, userID int64, token string) error
	Delete(ctx context.Context, token string) error
}

// Config carries the OIDC fields needed to configure the Handler.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string // origin of the SPA (e.g. http://localhost:5173); empty = same origin as backend
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
	users                 UserRepository
	sessions              SessionRepository
}

// NewHandler constructs a Handler from a discovered OIDC provider.
func NewHandler(provider *oidc.Provider, cfg Config, users UserRepository, sessions SessionRepository) *Handler {
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
		users:       users,
		sessions:    sessions,
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

	userID, err := h.users.UpsertUser(ctx, claims.Email, claims.Name)
	if err != nil {
		slog.Error("upsert user failed", "error", err)
		http.Error(w, "user persistence failed", http.StatusInternalServerError)
		return
	}

	if err := h.users.UpsertIdentity(ctx, userID, h.issuerURL, claims.Sub); err != nil {
		slog.Error("upsert identity failed", "error", err)
		http.Error(w, "identity persistence failed", http.StatusInternalServerError)
		return
	}

	sessionToken, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.Create(ctx, userID, sessionToken); err != nil {
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

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
