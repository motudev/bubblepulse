package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/motudev/bubblepulse/internal/api"
	"github.com/motudev/bubblepulse/internal/auth"
	"github.com/motudev/bubblepulse/internal/db"
	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/pkg/config"
	"github.com/pressly/goose/v3"
)

func main() {
	// Load .env if present; silently ignored in production where env vars are set directly.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	if err := goose.Up(sqlDB, "internal/db/migrations"); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	oidcProvider, err := auth.NewProvider(context.Background(), cfg.OIDCIssuerURL)
	if err != nil {
		slog.Error("OIDC provider discovery failed", "issuer", cfg.OIDCIssuerURL, "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)

	authHandler := auth.NewHandler(oidcProvider, auth.Config{
		IssuerURL:    cfg.OIDCIssuerURL,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		FrontendURL:  cfg.FrontendURL,
	}, userRepo, sessionRepo)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      api.New(authHandler, sessionRepo, userRepo),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
