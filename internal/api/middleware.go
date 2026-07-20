package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// ctxKey is an unexported type that prevents context key collisions across packages.
type ctxKey string

const (
	userIDKey      ctxKey = "userID"
	currentUserKey ctxKey = "currentUser"
)

// requireSession validates the session cookie, injects the user ID and the
// tenant (organization) ID into the request context, and returns 401 for
// missing, invalid, or expired sessions. In pooled mode a session without an
// organization (created before multi-tenancy) is rejected so the user
// re-authenticates and gets provisioned.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sess, err := s.sessions.FindByToken(r.Context(), cookie.Value)
		if err != nil {
			if !errors.Is(err, repository.ErrSessionNotFound) {
				slog.Error("session: DB error during lookup", "error", err)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, sess.UserID)
		switch {
		case sess.OrgID != nil:
			ctx = tenancy.WithTenantID(ctx, *sess.OrgID)
		case !s.runner.Siloed():
			// Pooled mode requires a tenant for every RLS-scoped query.
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(ctx))
	}
}

// requireRole guards a handler behind a session AND one of the given roles.
// The role is read from the users row on every request (inside a tenant
// transaction), so role changes take effect immediately without re-login.
// The full user record is stored in the context for handlers that need
// own-team scoping.
func (s *Server) requireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return s.requireSession(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var user repository.UserRecord
			err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
				var txErr error
				user, txErr = s.users.FindByID(r.Context(), tx, userID)
				return txErr
			})
			if err != nil {
				slog.Error("rbac: failed to load user role", "user_id", userID, "error", err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !slices.Contains(roles, user.Role) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), currentUserKey, user)
			next(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID injected by requireSession.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// CurrentUserFromContext extracts the full user record injected by requireRole.
func CurrentUserFromContext(ctx context.Context) (repository.UserRecord, bool) {
	u, ok := ctx.Value(currentUserKey).(repository.UserRecord)
	return u, ok
}
