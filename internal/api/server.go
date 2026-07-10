// Package api wires together the HTTP mux and all route handlers.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/motudev/dailybot/internal/auth"
)

// Server holds the configured HTTP mux.
type Server struct {
	mux *http.ServeMux
}

// New constructs a Server and registers all routes.
func New(authHandler *auth.Handler) *Server {
	s := &Server{mux: http.NewServeMux()}
	s.routes(authHandler)
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

func (s *Server) routes(auth *auth.Handler) {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/auth/login", auth.Login)
	s.mux.HandleFunc("GET /api/auth/callback", auth.Callback)
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
