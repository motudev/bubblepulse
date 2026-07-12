// Package config loads and validates application configuration from environment variables.
package config

import (
	"errors"
	"os"
)

// Config holds all runtime configuration for the application.
type Config struct {
	Port               string
	DatabaseURL        string
	OIDCIssuerURL      string
	OIDCClientID       string
	OIDCClientSecret   string
	OIDCRedirectURL    string
	FrontendURL        string // origin of the SPA; empty means same origin as the backend
	SlackSigningSecret string
	SlackBotToken      string
	ONNXRuntimePath    string // path to libonnxruntime shared library; defaults to "libonnxruntime.so"
	NLPServiceURL      string // base URL of the Python NLP sidecar; defaults to http://localhost:8090
}

// Load reads environment variables and returns a validated Config.
// Returns an error listing all missing required variables.
func Load() (Config, error) {
	cfg := Config{
		Port:               getEnvWithDefault("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		OIDCIssuerURL:      getEnvWithDefault("OIDC_ISSUER_URL", "https://slack.com"),
		OIDCClientID:       os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:   os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:    os.Getenv("OIDC_REDIRECT_URL"),
		FrontendURL:        os.Getenv("FRONTEND_URL"),
		SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		SlackBotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		ONNXRuntimePath:    getEnvWithDefault("ONNX_RUNTIME_PATH", "libonnxruntime.so"),
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
	if cfg.SlackBotToken == "" {
		missing = append(missing, "SLACK_BOT_TOKEN")
	}

	if len(missing) > 0 {
		return Config{}, errors.New("missing required environment variables: " + join(missing))
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
