package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
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
}

// Config carries the four OIDC fields needed to configure the Handler.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Handler holds the OIDC provider and wires the login/callback HTTP handlers.
type Handler struct {
	provider  *oidc.Provider
	oauth2Cfg oauth2.Config
	verifier  *oidc.IDTokenVerifier
	issuerURL string
	secure    bool
	users     UserRepository
	sessions  SessionRepository
}

// NewHandler constructs a Handler from a discovered OIDC provider.
func NewHandler(provider *oidc.Provider, cfg Config, users UserRepository, sessions SessionRepository) *Handler {
	return &Handler{
		provider: provider,
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier:  provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		issuerURL: cfg.IssuerURL,
		secure:    strings.HasPrefix(cfg.RedirectURL, "https://"),
		users:     users,
		sessions:  sessions,
	}
}

// Login generates a CSRF state token, stores it in an HTTP-only cookie, and
// redirects the user to the OIDC provider's authorization endpoint.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/api/auth/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.oauth2Cfg.AuthCodeURL(state), http.StatusFound)
}

// Callback verifies the CSRF state, exchanges the authorization code for tokens,
// verifies the id_token, upserts the user and identity, creates a session, and
// redirects to the frontend root.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || stateCookie.Value != r.FormValue("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Path:     "/api/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})

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

	http.Redirect(w, r, "/", http.StatusFound)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
