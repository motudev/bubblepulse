package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// dashboardQuerier is the read side of the daily update repository.
type dashboardQuerier interface {
	FindLatestPerUser(ctx context.Context) ([]repository.DashboardRow, error)
}

type dashboardHandler struct {
	repo dashboardQuerier
}

// NewDashboardHandler constructs a dashboardHandler.
func NewDashboardHandler(repo dashboardQuerier) *dashboardHandler {
	return &dashboardHandler{repo: repo}
}

// dashboardEntry is the JSON shape for a single user entry on the dashboard.
type dashboardEntry struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	UpdateText *string    `json:"update_text"`
	UpdateAt   *time.Time `json:"update_at"`
}

// handleDashboard handles GET /api/dashboard.
func (h *dashboardHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.FindLatestPerUser(r.Context())
	if err != nil {
		slog.Error("dashboard query failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	entries := make([]dashboardEntry, len(rows))
	for i, row := range rows {
		entries[i] = dashboardEntry{
			ID:         row.UserID,
			Name:       row.Name,
			Email:      row.Email,
			UpdateText: row.UpdateText,
			UpdateAt:   row.UpdateAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}
