// Package auth implements OIDC-based authentication.
package auth

import (
	"context"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// UserClaims holds normalized identity fields extracted from the id_token.
type UserClaims struct {
	Sub     string
	Email   string
	Name    string
	Picture string
	// WorkspaceID is the provider's tenant identifier (Slack team_id; a future
	// Teams/Google/SAML tenant handshake maps its tenant ID here). Empty when
	// the provider exposes no workspace concept.
	WorkspaceID string
	// WorkspaceName is the human-readable workspace name, when the provider
	// supplies one. Used to pre-fill the organization name at auto-provisioning.
	WorkspaceName string
}

// NewProvider discovers OIDC endpoints from the issuer's well-known configuration URL.
func NewProvider(ctx context.Context, issuerURL string) (*oidc.Provider, error) {
	return oidc.NewProvider(ctx, issuerURL)
}

// normalizeClaims maps raw JWT claims to UserClaims, handling per-provider field differences.
func normalizeClaims(issuerURL string, raw map[string]interface{}) UserClaims {
	claims := UserClaims{
		Sub:   strClaim(raw, "sub"),
		Email: strClaim(raw, "email"),
		Name:  strClaim(raw, "name"),
	}
	switch {
	case strings.Contains(issuerURL, "slack.com"):
		claims.Picture = strClaim(raw, "image_192")
		claims.WorkspaceID = strClaim(raw, "https://slack.com/team_id")
		claims.WorkspaceName = strClaim(raw, "https://slack.com/team_name")
	default:
		claims.Picture = strClaim(raw, "picture")
	}
	return claims
}

func strClaim(raw map[string]interface{}, key string) string {
	v, _ := raw[key].(string)
	return v
}
