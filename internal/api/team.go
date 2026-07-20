package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// teamEntry is the team JSON shape in API responses.
type teamEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type teamRequest struct {
	Name string `json:"name"`
}

// handleListTeams handles GET /api/v1/teams. Any authenticated member may
// list their organization's teams (needed for the dashboard team scope).
func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	var teams []repository.TeamRecord
	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		var txErr error
		teams, txErr = s.teams.ListTeams(r.Context(), tx)
		return txErr
	})
	if err != nil {
		slog.Error("teams: list failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	entries := make([]teamEntry, len(teams))
	for i, t := range teams {
		entries[i] = teamEntry{ID: t.ID, Name: t.Name}
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleCreateTeam handles POST /api/v1/teams (ADMIN only).
func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUserFromContext(r.Context())
	if !ok || user.OrgID == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req teamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	var id string
	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		var txErr error
		id, txErr = s.teams.CreateTeam(r.Context(), tx, *user.OrgID, strings.TrimSpace(req.Name))
		return txErr
	})
	if err != nil {
		slog.Error("teams: create failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, teamEntry{ID: id, Name: strings.TrimSpace(req.Name)})
}

// handleUpdateTeam handles PATCH /api/v1/teams/{id} (ADMIN, or TEAM_EDITOR
// for their own team).
func (s *Server) handleUpdateTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUserFromContext(r.Context())
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	teamID := r.PathValue("id")
	if !tenancy.IsValidUUID(teamID) {
		http.Error(w, "invalid team id", http.StatusBadRequest)
		return
	}
	if user.Role == repository.RoleTeamEditor && (user.TeamID == nil || *user.TeamID != teamID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req teamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		return s.teams.RenameTeam(r.Context(), tx, teamID, strings.TrimSpace(req.Name))
	})
	if errors.Is(err, repository.ErrTeamNotFound) {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("teams: rename failed", "team_id", teamID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, teamEntry{ID: teamID, Name: strings.TrimSpace(req.Name)})
}

// handleDeleteTeam handles DELETE /api/v1/teams/{id} (ADMIN only). Members of
// the deleted team fall back to no team (fk_users_team ON DELETE SET NULL).
func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")
	if !tenancy.IsValidUUID(teamID) {
		http.Error(w, "invalid team id", http.StatusBadRequest)
		return
	}

	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		return s.teams.DeleteTeam(r.Context(), tx, teamID)
	})
	if errors.Is(err, repository.ErrTeamNotFound) {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("teams: delete failed", "team_id", teamID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
