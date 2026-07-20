package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// dashboardQuerier is the read side of the daily update repository.
// Both methods touch RLS tables and take a tenant-scoped Querier.
type dashboardQuerier interface {
	FindLatestPerUserWithTopics(ctx context.Context, q repository.Querier, teamID *string) ([]repository.DashboardRowWithTopics, error)
	FindTodayTopicSimilarities(ctx context.Context, q repository.Querier, teamID *string) ([]repository.TopicSimilarityRow, error)
}

type dashboardHandler struct {
	runner tenantTxRunner
	repo   dashboardQuerier
}

// NewDashboardHandler constructs a dashboardHandler.
func NewDashboardHandler(runner tenantTxRunner, repo dashboardQuerier) *dashboardHandler {
	return &dashboardHandler{runner: runner, repo: repo}
}

// userEntry is the per-user JSON shape in the dashboard response.
type userEntry struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	UpdateText *string    `json:"update_text"`
	UpdateAt   *time.Time `json:"update_at"`
	Topics     []string   `json:"topics"`
}

// dashboardResponse is the top-level JSON shape for GET /api/dashboard.
type dashboardResponse struct {
	Users            []userEntry `json:"users"`
	Topics           []string    `json:"topics"`
	SimilarityMatrix [][]float64 `json:"similarity_matrix"`
}

// handleDashboard handles GET /api/dashboard. An optional ?team_id= query
// parameter restricts the result to one team; the RLS policies scope
// everything to the caller's organization.
func (h *dashboardHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	var teamID *string
	if v := r.URL.Query().Get("team_id"); v != "" {
		if !tenancy.IsValidUUID(v) {
			http.Error(w, "invalid team_id", http.StatusBadRequest)
			return
		}
		teamID = &v
	}

	var rows []repository.DashboardRowWithTopics
	var sims []repository.TopicSimilarityRow
	err := h.runner.RunTx(r.Context(), func(tx pgx.Tx) error {
		var txErr error
		if rows, txErr = h.repo.FindLatestPerUserWithTopics(r.Context(), tx, teamID); txErr != nil {
			return txErr
		}
		sims, txErr = h.repo.FindTodayTopicSimilarities(r.Context(), tx, teamID)
		return txErr
	})
	if err != nil {
		slog.Error("dashboard query failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build a sorted, deduplicated topic list for stable matrix index alignment.
	seen := make(map[string]struct{})
	var topics []string
	for _, row := range rows {
		for _, t := range row.Topics {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				topics = append(topics, t)
			}
		}
	}
	sort.Strings(topics)

	// Build topic-to-index map and NxN similarity matrix.
	topicIdx := make(map[string]int, len(topics))
	for i, t := range topics {
		topicIdx[t] = i
	}

	n := len(topics)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		if n > 0 {
			matrix[i][i] = 1.0
		}
	}
	for _, sim := range sims {
		i, iOK := topicIdx[sim.TopicA]
		j, jOK := topicIdx[sim.TopicB]
		if iOK && jOK {
			matrix[i][j] = sim.Similarity
			matrix[j][i] = sim.Similarity
		}
	}

	// Assemble user entries with a guaranteed non-nil Topics slice.
	users := make([]userEntry, len(rows))
	for k, row := range rows {
		t := row.Topics
		if t == nil {
			t = []string{}
		}
		users[k] = userEntry{
			ID:         row.UserID,
			Name:       row.Name,
			Email:      row.Email,
			UpdateText: row.UpdateText,
			UpdateAt:   row.UpdateAt,
			Topics:     t,
		}
	}

	if topics == nil {
		topics = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dashboardResponse{
		Users:            users,
		Topics:           topics,
		SimilarityMatrix: matrix,
	})
}
