package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// dashboardQuerier is the read side of the daily update repository.
type dashboardQuerier interface {
	FindLatestPerUserWithTopics(ctx context.Context) ([]repository.DashboardRowWithTopics, error)
	FindTodayTopicSimilarities(ctx context.Context) ([]repository.TopicSimilarityRow, error)
}

type dashboardHandler struct {
	repo dashboardQuerier
}

// NewDashboardHandler constructs a dashboardHandler.
func NewDashboardHandler(repo dashboardQuerier) *dashboardHandler {
	return &dashboardHandler{repo: repo}
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

// handleDashboard handles GET /api/dashboard.
func (h *dashboardHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.FindLatestPerUserWithTopics(r.Context())
	if err != nil {
		slog.Error("dashboard query failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sims, err := h.repo.FindTodayTopicSimilarities(r.Context())
	if err != nil {
		slog.Error("dashboard similarity query failed", "error", err)
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
