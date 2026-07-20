package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// orgUserEntry is the user JSON shape in the management API.
type orgUserEntry struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Role   string  `json:"role"`
	TeamID *string `json:"team_id"`
}

// updateUserRequest is the PATCH /api/v1/users/{id} body. Absent fields are
// left unchanged; team_id may be explicitly null to remove a team assignment.
type updateUserRequest struct {
	TeamID jsonNullableString `json:"team_id"`
	Role   *string            `json:"role,omitempty"`
}

// jsonNullableString distinguishes an absent JSON field from an explicit null.
// Present is true when the key appeared in the JSON payload; Value is nil when
// the JSON value was null (meaning "remove assignment").
type jsonNullableString struct {
	Present bool
	Value   *string
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *jsonNullableString) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(data, &n.Value)
}

// handleListUsers handles GET /api/v1/users (ADMIN or TEAM_EDITOR).
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	var users []repository.UserRecord
	err := s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		var txErr error
		users, txErr = s.users.ListByOrg(r.Context(), tx)
		return txErr
	})
	if err != nil {
		slog.Error("users: list failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	entries := make([]orgUserEntry, len(users))
	for i, u := range users {
		entries[i] = orgUserEntry{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, TeamID: u.TeamID}
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleUpdateUser handles PATCH /api/v1/users/{id} (ADMIN or TEAM_EDITOR).
// ADMIN may assign any team and change roles (except demoting the last
// admin). TEAM_EDITOR may only move unassigned users into their own team or
// remove members from it, and may not touch roles.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUserFromContext(r.Context())
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !req.TeamID.Present && req.Role == nil {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}
	if req.Role != nil && actor.Role != repository.RoleAdmin {
		http.Error(w, "only admins may change roles", http.StatusForbidden)
		return
	}
	if req.Role != nil && !repository.IsValidRole(*req.Role) {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	if req.TeamID.Present && req.TeamID.Value != nil && !tenancy.IsValidUUID(*req.TeamID.Value) {
		http.Error(w, "invalid team id", http.StatusBadRequest)
		return
	}

	var updated repository.UserRecord
	err = s.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		target, txErr := s.users.FindByID(r.Context(), tx, targetID)
		if txErr != nil {
			return txErr
		}

		if req.TeamID.Present {
			if txErr := s.applyTeamChange(r, tx, actor, target, req.TeamID.Value); txErr != nil {
				return txErr
			}
		}

		if req.Role != nil && *req.Role != target.Role {
			// Protect against locking the organization out of administration.
			if target.Role == repository.RoleAdmin {
				admins, txErr := s.users.CountAdmins(r.Context(), tx)
				if txErr != nil {
					return txErr
				}
				if admins <= 1 {
					return errLastAdmin
				}
			}
			if txErr := s.users.SetRole(r.Context(), tx, targetID, *req.Role); txErr != nil {
				return txErr
			}
		}

		updated, txErr = s.users.FindByID(r.Context(), tx, targetID)
		return txErr
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "user not found", http.StatusNotFound)
		return
	case errors.Is(err, errLastAdmin):
		http.Error(w, "cannot demote the last admin", http.StatusConflict)
		return
	case errors.Is(err, errTeamForbidden):
		http.Error(w, "forbidden team change", http.StatusForbidden)
		return
	case errors.Is(err, repository.ErrTeamNotFound):
		http.Error(w, "team not found", http.StatusNotFound)
		return
	case err != nil:
		slog.Error("users: update failed", "user_id", targetID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, orgUserEntry{ID: updated.ID, Name: updated.Name, Email: updated.Email, Role: updated.Role, TeamID: updated.TeamID})
}

var (
	errLastAdmin     = errors.New("cannot demote the last admin")
	errTeamForbidden = errors.New("team change not permitted for this role")
)

// applyTeamChange enforces the per-role team assignment rules and performs
// the update. newTeamID nil means "remove from team".
func (s *Server) applyTeamChange(r *http.Request, tx pgx.Tx, actor, target repository.UserRecord, newTeamID *string) error {
	if actor.Role == repository.RoleTeamEditor {
		ownTeam := actor.TeamID
		if ownTeam == nil {
			return errTeamForbidden
		}
		removingOwnMember := newTeamID == nil && target.TeamID != nil && *target.TeamID == *ownTeam
		addingUnassigned := newTeamID != nil && *newTeamID == *ownTeam && target.TeamID == nil
		if !removingOwnMember && !addingUnassigned {
			return errTeamForbidden
		}
	}

	if newTeamID != nil {
		// Foreign-key validation bypasses RLS, so verify the team is visible
		// in this tenant before assigning it.
		if _, err := s.teams.FindTeamByID(r.Context(), tx, *newTeamID); err != nil {
			return err
		}
	}
	if err := s.users.SetTeam(r.Context(), tx, target.ID, newTeamID); err != nil {
		return fmt.Errorf("set team: %w", err)
	}
	return nil
}
