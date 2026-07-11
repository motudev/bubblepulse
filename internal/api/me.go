package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// SessionLookup validates opaque session tokens.
type SessionLookup interface {
	FindUserIDByToken(ctx context.Context, token string) (int64, error)
}

// UserLookup fetches user records by primary key.
type UserLookup interface {
	FindByID(ctx context.Context, id int64) (repository.UserRecord, error)
}

type meResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := s.users.FindByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meResponse{ID: u.ID, Email: u.Email, Name: u.Name})
}
