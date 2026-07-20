package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// orgInfo is the organization JSON shape embedded in API responses.
type orgInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type meResponse struct {
	ID     int64    `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	TeamID *string  `json:"team_id"`
	Org    *orgInfo `json:"org"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var u repository.UserRecord
	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		var txErr error
		u, txErr = s.users.FindByID(r.Context(), tx, userID)
		return txErr
	})
	if err != nil {
		slog.Error("me: failed to fetch user", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := meResponse{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, TeamID: u.TeamID}
	if u.OrgID != nil {
		org, err := s.orgs.FindOrgByID(r.Context(), *u.OrgID)
		if err != nil {
			slog.Error("me: failed to fetch organization", "org_id", *u.OrgID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp.Org = &orgInfo{ID: org.ID, Name: org.Name}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
