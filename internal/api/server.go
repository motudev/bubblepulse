// Package api wires together the HTTP mux and all route handlers.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/auth"
	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/messaging"
)

// SessionLookup validates opaque session tokens against the Global Directory.
type SessionLookup interface {
	FindByToken(ctx context.Context, token string) (repository.SessionRecord, error)
}

// UserStore is the RLS-scoped user access the API layer needs.
type UserStore interface {
	FindByID(ctx context.Context, q repository.Querier, id int64) (repository.UserRecord, error)
	ListByOrg(ctx context.Context, q repository.Querier) ([]repository.UserRecord, error)
	SetTeam(ctx context.Context, q repository.Querier, userID int64, teamID *string) error
	SetRole(ctx context.Context, q repository.Querier, userID int64, role string) error
	CountAdmins(ctx context.Context, q repository.Querier) (int, error)
}

// TeamStore is the RLS-scoped team access the API layer needs.
type TeamStore interface {
	ListTeams(ctx context.Context, q repository.Querier) ([]repository.TeamRecord, error)
	FindTeamByID(ctx context.Context, q repository.Querier, id string) (repository.TeamRecord, error)
	CreateTeam(ctx context.Context, q repository.Querier, orgID, name string) (string, error)
	RenameTeam(ctx context.Context, q repository.Querier, teamID, name string) error
	DeleteTeam(ctx context.Context, q repository.Querier, teamID string) error
}

// OrgStore is the Global Directory organization access the API layer needs.
type OrgStore interface {
	FindOrgByID(ctx context.Context, id string) (repository.OrgRecord, error)
	RenameOrg(ctx context.Context, id, name string) error
}

// tenantTxRunner opens tenant-scoped transactions (satisfied by *tenancy.Runner).
type tenantTxRunner interface {
	RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error
	Siloed() bool
}

// Server holds the configured HTTP mux and its dependencies.
type Server struct {
	mux                 *http.ServeMux
	runner              tenantTxRunner
	sessions            SessionLookup
	users               UserStore
	teams               TeamStore
	orgs                OrgStore
	slackInstallEnabled bool
	slackOIDC           bool
}

// Deps bundles the dependencies of the API server.
type Deps struct {
	Runner              tenantTxRunner
	Sessions            SessionLookup
	Users               UserStore
	Teams               TeamStore
	Orgs                OrgStore
	SlackInstallEnabled bool // true when SLACK_CLIENT_ID + SLACK_CLIENT_SECRET are configured
	SlackOIDC           bool // true when OIDC_ISSUER_URL is the Slack issuer
}

// New constructs a Server and registers all routes.
func New(authHandler *auth.Handler, deps Deps, platforms []messaging.PlatformAdapter, dashH *dashboardHandler) *Server {
	s := &Server{
		mux:                 http.NewServeMux(),
		runner:              deps.Runner,
		sessions:            deps.Sessions,
		users:               deps.Users,
		teams:               deps.Teams,
		orgs:                deps.Orgs,
		slackInstallEnabled: deps.SlackInstallEnabled,
		slackOIDC:           deps.SlackOIDC,
	}
	s.routes(authHandler, platforms, dashH)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(lrw, r)
	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", lrw.statusCode,
		"duration", time.Since(start).String(),
	)
}

func (s *Server) routes(auth *auth.Handler, platforms []messaging.PlatformAdapter, dashH *dashboardHandler) {
	adminOrEditor := s.requireRole(repository.RoleAdmin, repository.RoleTeamEditor)
	adminOnly := s.requireRole(repository.RoleAdmin)

	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/auth/login", auth.Login)
	s.mux.HandleFunc("GET /api/auth/callback", auth.Callback)
	s.mux.HandleFunc("GET /api/auth/logout", auth.Logout)
	s.mux.HandleFunc("GET /api/v1/me", s.requireSession(s.handleMe))
	s.mux.HandleFunc("GET /api/dashboard", s.requireSession(dashH.handleDashboard))

	s.mux.HandleFunc("GET /api/v1/teams", s.requireSession(s.handleListTeams))
	s.mux.HandleFunc("POST /api/v1/teams", adminOnly(s.handleCreateTeam))
	s.mux.HandleFunc("PATCH /api/v1/teams/{id}", adminOrEditor(s.handleUpdateTeam))
	s.mux.HandleFunc("DELETE /api/v1/teams/{id}", adminOnly(s.handleDeleteTeam))

	s.mux.HandleFunc("GET /api/v1/users", adminOrEditor(s.handleListUsers))
	s.mux.HandleFunc("PATCH /api/v1/users/{id}", adminOrEditor(s.handleUpdateUser))

	s.mux.HandleFunc("PATCH /api/v1/org", adminOnly(s.handleUpdateOrg))

	for _, p := range platforms {
		s.mux.HandleFunc(p.Route(), p.Handler())
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// loggingResponseWriter captures the HTTP status code for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
