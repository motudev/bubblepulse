package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// ctxKey is an unexported type that prevents context key collisions across packages.
type ctxKey string

const userIDKey ctxKey = "userID"

// requireSession validates the session cookie and injects the user ID into the request context.
// Returns 401 for missing, invalid, or expired sessions.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := s.sessions.FindUserIDByToken(r.Context(), cookie.Value)
		if err != nil {
			if !errors.Is(err, repository.ErrSessionNotFound) {
				slog.Error("session: DB error during lookup", "error", err)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserIDFromContext extracts the authenticated user ID injected by requireSession.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}
