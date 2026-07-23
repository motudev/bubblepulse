// Package config loads and validates application configuration from environment variables.
package config

import (
	"errors"
	"log/slog"
	"os"
)

// Tenancy deployment modes selectable via TENANCY_MODE.
const (
	// TenancyPooled runs the app as shared multi-tenant SaaS with RLS enforcement.
	TenancyPooled = "pooled"
	// TenancySiloed runs the app as a dedicated single-tenant deployment,
	// activating the RLS bypass condition baked into every policy.
	TenancySiloed = "siloed"
)

// Config holds all runtime configuration for the application.
type Config struct {
	Port                    string
	TenancyMode             string // TenancyPooled (default) or TenancySiloed
	DatabaseURL             string
	OIDCIssuerURL           string
	OIDCClientID            string
	OIDCClientSecret        string
	OIDCRedirectURL         string
	FrontendURL             string // origin of the SPA; empty means same origin as the backend
	SlackSigningSecret      string // per-app; verifies every Events API webhook
	SlackBotToken           string // siloed-mode fallback; in pooled mode tokens come from platform_workspaces
	SlackClientID           string // required to enable the OAuth install flow
	SlackClientSecret       string // required to enable the OAuth install flow
	SlackInstallRedirectURL string // redirect URI registered in the Slack app settings
	ONNXRuntimePath         string // path to libonnxruntime shared library; defaults to "libonnxruntime.so"
	NLPServiceURL           string // base URL of the Python NLP sidecar; defaults to http://localhost:8090
}

// Load reads environment variables and returns a validated Config.
// Returns an error listing all missing required variables.
func Load() (Config, error) {
	cfg := Config{
		Port:               getEnvWithDefault("PORT", "8080"),
		TenancyMode:        getEnvWithDefault("TENANCY_MODE", TenancyPooled),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		OIDCIssuerURL:      getEnvWithDefault("OIDC_ISSUER_URL", "https://slack.com"),
		OIDCClientID:       os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:   os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:    os.Getenv("OIDC_REDIRECT_URL"),
		FrontendURL:        os.Getenv("FRONTEND_URL"),
		SlackSigningSecret:      os.Getenv("SLACK_SIGNING_SECRET"),
		SlackBotToken:           os.Getenv("SLACK_BOT_TOKEN"),
		SlackClientID:           os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:       os.Getenv("SLACK_CLIENT_SECRET"),
		SlackInstallRedirectURL: os.Getenv("SLACK_INSTALL_REDIRECT_URL"),
		ONNXRuntimePath:         getEnvWithDefault("ONNX_RUNTIME_PATH", "libonnxruntime.so"),
		NLPServiceURL:      getEnvWithDefault("NLP_SERVICE_URL", "http://localhost:8090"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.OIDCClientID == "" {
		missing = append(missing, "OIDC_CLIENT_ID")
	}
	if cfg.OIDCClientSecret == "" {
		missing = append(missing, "OIDC_CLIENT_SECRET")
	}
	if cfg.OIDCRedirectURL == "" {
		missing = append(missing, "OIDC_REDIRECT_URL")
	}
	if cfg.SlackSigningSecret == "" {
		missing = append(missing, "SLACK_SIGNING_SECRET")
	}

	var problems []string
	if len(missing) > 0 {
		problems = append(problems, "missing required environment variables: "+join(missing))
	}
	if cfg.TenancyMode != TenancyPooled && cfg.TenancyMode != TenancySiloed {
		problems = append(problems, "TENANCY_MODE must be '"+TenancyPooled+"' or '"+TenancySiloed+"', got '"+cfg.TenancyMode+"'")
	}

	if len(problems) > 0 {
		return Config{}, errors.New(join(problems))
	}

	if cfg.TenancyMode == TenancyPooled && (cfg.SlackClientID == "" || cfg.SlackClientSecret == "") {
		slog.Warn("SLACK_CLIENT_ID or SLACK_CLIENT_SECRET not set: Slack OAuth install flow disabled; new workspaces cannot install the bot")
	}

	return cfg, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func join(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
