package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	allminilm "github.com/clems4ever/all-minilm-l6-v2-go/all_minilm_l6_v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/motudev/bubblepulse/internal/api"
	"github.com/motudev/bubblepulse/internal/auth"
	"github.com/motudev/bubblepulse/internal/db"
	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/messaging"
	slackplatform "github.com/motudev/bubblepulse/internal/platform/slack"
	"github.com/motudev/bubblepulse/internal/tenancy"
	"github.com/motudev/bubblepulse/internal/worker"
	"github.com/motudev/bubblepulse/pkg/config"
)

// normalizedModel adapts *allminilm.Model to the embeddingComputer interface
// expected by worker.NewNLPWorker, always producing L2-normalised vectors.
type normalizedModel struct{ m *allminilm.Model }

func (n *normalizedModel) Compute(text string) ([]float32, error) {
	return n.m.Compute(text, true)
}

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

	// Tenant transaction runner: binds every transaction to the caller's
	// organization (pooled) or to the RLS bypass (siloed).
	siloed := cfg.TenancyMode == config.TenancySiloed
	runner := tenancy.NewRunner(pool, siloed)
	if !siloed {
		// A superuser or BYPASSRLS role would silently skip every RLS policy,
		// exposing all tenants to each other — refuse to start.
		if err := tenancy.VerifyPooledSafety(context.Background(), pool); err != nil {
			slog.Error("pooled tenancy safety check failed", "error", err)
			os.Exit(1)
		}
	}

	// Run River's own schema migrations (idempotent; tracks applied versions in river_migration).
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		slog.Error("river migrator init failed", "error", err)
		os.Exit(1)
	}
	if _, err := migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil); err != nil {
		slog.Error("river migrations failed", "error", err)
		os.Exit(1)
	}

	// Initialise the ONNX sentence-transformer model (requires CGO_ENABLED=1 and libonnxruntime.so).
	embedder, err := allminilm.NewModel(allminilm.WithRuntimePath(cfg.ONNXRuntimePath))
	if err != nil {
		slog.Error("ONNX embedder init failed", "path", cfg.ONNXRuntimePath, "error", err)
		os.Exit(1)
	}
	defer embedder.Close()

	// Register River workers.
	nlpClient := worker.NewNLPServiceClient(cfg.NLPServiceURL)
	dailyUpdateRepo := repository.NewDailyUpdateRepo(pool)
	workers := river.NewWorkers()
	river.AddWorker(workers, worker.NewNLPWorker(runner, dailyUpdateRepo, &normalizedModel{embedder}, nlpClient))

	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers: workers,
	})
	if err != nil {
		slog.Error("river client init failed", "error", err)
		os.Exit(1)
	}

	oidcProvider, err := auth.NewProvider(context.Background(), cfg.OIDCIssuerURL)
	if err != nil {
		slog.Error("OIDC provider discovery failed", "issuer", cfg.OIDCIssuerURL, "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	orgRepo := repository.NewOrgRepo(pool)
	workspaceRepo := repository.NewWorkspaceRepo(pool)
	teamRepo := repository.NewTeamRepo(pool)

	authHandler := auth.NewHandler(oidcProvider, auth.Config{
		IssuerURL:    cfg.OIDCIssuerURL,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		FrontendURL:  cfg.FrontendURL,
	}, pool, runner, auth.Repos{
		Users:      userRepo,
		Sessions:   sessionRepo,
		Orgs:       orgRepo,
		Workspaces: workspaceRepo,
	})

	// Platform-agnostic message service wires the Global Directory resolution,
	// tenant-scoped DB writes, and queue logic.
	msgSvc := messaging.NewMessageService(runner, userRepo, workspaceRepo, dailyUpdateRepo, riverClient)

	// Register platform adapters — add new platforms here without touching anything else.
	platforms := []messaging.PlatformAdapter{}
	if cfg.SlackSigningSecret != "" {
		platforms = append(platforms, slackplatform.NewAdapter(cfg.SlackSigningSecret, msgSvc))
	}

	// Register the Slack OAuth install flow when client credentials are configured.
	// In siloed mode this is optional; SLACK_BOT_TOKEN can be used as a fallback instead.
	if cfg.SlackClientID != "" && cfg.SlackClientSecret != "" {
		installer := slackplatform.NewInstaller(
			cfg.SlackClientID,
			cfg.SlackClientSecret,
			cfg.SlackInstallRedirectURL,
			cfg.FrontendURL,
			pool,
			orgRepo,
			workspaceRepo,
		)
		platforms = append(platforms, installer.Adapters()...)
	}

	dashH := api.NewDashboardHandler(runner, dailyUpdateRepo)

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: api.New(authHandler, api.Deps{
			Runner:              runner,
			Sessions:            sessionRepo,
			Users:               userRepo,
			Teams:               teamRepo,
			Orgs:                orgRepo,
			SlackInstallEnabled: cfg.SlackClientID != "" && cfg.SlackClientSecret != "",
			SlackOIDC:           strings.HasPrefix(cfg.OIDCIssuerURL, "https://slack.com"),
		}, platforms, dashH),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("starting River worker", "queues", []string{river.QueueDefault})
		if err := riverClient.Start(context.Background()); err != nil {
			slog.Error("river worker error", "error", err)
		}
	}()

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

	if err := riverClient.Stop(ctx); err != nil {
		slog.Error("river shutdown error", "error", err)
	}

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
