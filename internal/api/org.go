package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

type updateOrgRequest struct {
	Name string `json:"name"`
}

// handleUpdateOrg handles PATCH /api/v1/org (ADMIN only): renames the
// caller's organization. Used in particular to name auto-provisioned
// organizations whose name could not be inferred at first login.
func (s *Server) handleUpdateOrg(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUserFromContext(r.Context())
	if !ok || user.OrgID == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req updateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	err := s.orgs.RenameOrg(r.Context(), *user.OrgID, name)
	if errors.Is(err, repository.ErrOrgNotFound) {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("org: rename failed", "org_id", *user.OrgID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orgInfo{ID: *user.OrgID, Name: name})
}
