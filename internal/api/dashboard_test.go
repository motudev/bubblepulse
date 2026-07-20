package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

func dashboardRequest(target string) *http.Request {
	return requestAs(adminIn(orgA), http.MethodGet, target, "")
}

// TestHandleDashboard_TeamIDValidation partitions the team_id query parameter:
// absent, valid UUID, and malformed values that must never reach the repository.
func TestHandleDashboard_TeamIDValidation(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantTeamID *string // teamID the repository must receive on 200
	}{
		{"absent_team_id_queries_whole_org", "/api/dashboard", http.StatusOK, nil},
		{"valid_team_id_is_passed_through", "/api/dashboard?team_id=" + teamA, http.StatusOK, strPtr(teamA)},
		{"malformed_team_id_returns_400", "/api/dashboard?team_id=not-a-uuid", http.StatusBadRequest, nil},
		{"team_id_one_char_short_returns_400", "/api/dashboard?team_id=" + teamA[:35], http.StatusBadRequest, nil},
		{"injection_shaped_team_id_returns_400", "/api/dashboard?team_id=%27%3B%20DROP%20TABLE%20teams%3B%20--", http.StatusBadRequest, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeDashboardRepo{}
			h := NewDashboardHandler(&fakeRunner{}, repo)

			rec := httptest.NewRecorder()
			h.handleDashboard(rec, dashboardRequest(tc.target))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest {
				if len(repo.gotTeamIDs) != 0 {
					t.Fatalf("repository must not be queried on invalid team_id, got %+v", repo.gotTeamIDs)
				}
				return
			}
			if len(repo.gotTeamIDs) != 1 {
				t.Fatalf("repository queried %d times, want 1", len(repo.gotTeamIDs))
			}
			got := repo.gotTeamIDs[0]
			switch {
			case tc.wantTeamID == nil && got != nil:
				t.Fatalf("repository received team_id %q, want nil", *got)
			case tc.wantTeamID != nil && (got == nil || *got != *tc.wantTeamID):
				t.Fatalf("repository received team_id %v, want %q", got, *tc.wantTeamID)
			}
		})
	}
}

// TestHandleDashboard_EmptyPayloadShape pins the JSON contract for an org with
// no data: arrays must serialize as [], never null — the SPA iterates them
// without guards.
func TestHandleDashboard_EmptyPayloadShape(t *testing.T) {
	h := NewDashboardHandler(&fakeRunner{}, &fakeDashboardRepo{})

	rec := httptest.NewRecorder()
	h.handleDashboard(rec, dashboardRequest("/api/dashboard"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"users":[]`, `"topics":[]`, `"similarity_matrix":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s (got: %s)", want, body)
		}
	}
	if strings.Contains(body, "null") {
		t.Errorf("empty payload must not contain null arrays: %s", body)
	}
}

// TestHandleDashboard_MatrixAssembly verifies topic dedup/sorting and the
// similarity matrix invariants: diagonal 1.0, symmetric fill, 0.0 for missing
// pairs, and unknown topics in similarity rows ignored.
func TestHandleDashboard_MatrixAssembly(t *testing.T) {
	now := time.Now()
	repo := &fakeDashboardRepo{
		rows: []repository.DashboardRowWithTopics{
			{UserID: 1, Name: "Alice", Email: "alice@a.test", UpdateText: strPtr("did x"), UpdateAt: &now,
				Topics: []string{"ship auth", "deploy api"}},
			{UserID: 2, Name: "Bob", Email: "bob@a.test", UpdateText: strPtr("did y"), UpdateAt: &now,
				Topics: []string{"deploy api", "write docs"}}, // duplicate topic across users
			{UserID: 3, Name: "Carol", Email: "carol@a.test"}, // no update today
		},
		sims: []repository.TopicSimilarityRow{
			{TopicA: "deploy api", TopicB: "ship auth", Similarity: 0.91},
			{TopicA: "ghost topic", TopicB: "ship auth", Similarity: 0.99}, // not in any user's topics
		},
	}
	h := NewDashboardHandler(&fakeRunner{}, repo)

	rec := httptest.NewRecorder()
	h.handleDashboard(rec, dashboardRequest("/api/dashboard"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	wantTopics := []string{"deploy api", "ship auth", "write docs"} // sorted, deduplicated
	if len(resp.Topics) != len(wantTopics) {
		t.Fatalf("topics = %v, want %v", resp.Topics, wantTopics)
	}
	for i, topic := range wantTopics {
		if resp.Topics[i] != topic {
			t.Fatalf("topics = %v, want %v", resp.Topics, wantTopics)
		}
	}

	n := len(wantTopics)
	if len(resp.SimilarityMatrix) != n {
		t.Fatalf("matrix has %d rows, want %d", len(resp.SimilarityMatrix), n)
	}
	for i := 0; i < n; i++ {
		if len(resp.SimilarityMatrix[i]) != n {
			t.Fatalf("matrix row %d has %d columns, want %d", i, len(resp.SimilarityMatrix[i]), n)
		}
		if resp.SimilarityMatrix[i][i] != 1.0 {
			t.Errorf("matrix diagonal [%d][%d] = %v, want 1.0", i, i, resp.SimilarityMatrix[i][i])
		}
		for j := 0; j < n; j++ {
			if resp.SimilarityMatrix[i][j] != resp.SimilarityMatrix[j][i] {
				t.Errorf("matrix not symmetric at [%d][%d]", i, j)
			}
		}
	}
	// "deploy api"(0) ↔ "ship auth"(1) came from the similarity rows.
	if resp.SimilarityMatrix[0][1] != 0.91 {
		t.Errorf("matrix[0][1] = %v, want 0.91", resp.SimilarityMatrix[0][1])
	}
	// "deploy api"(0) ↔ "write docs"(2) has no similarity row → 0.0.
	if resp.SimilarityMatrix[0][2] != 0.0 {
		t.Errorf("matrix[0][2] = %v, want 0.0 for missing pair", resp.SimilarityMatrix[0][2])
	}

	// A user without an update today still appears, with nulls and an empty
	// (not null) topics array.
	if len(resp.Users) != 3 {
		t.Fatalf("users = %d, want 3", len(resp.Users))
	}
	carol := resp.Users[2]
	if carol.UpdateText != nil || carol.UpdateAt != nil {
		t.Errorf("user without update must have null update fields, got %+v", carol)
	}
	if carol.Topics == nil || len(carol.Topics) != 0 {
		t.Errorf("user without update must have empty topics array, got %v", carol.Topics)
	}
}

// TestHandleDashboard_Failures covers the repository and tenancy failure paths.
func TestHandleDashboard_Failures(t *testing.T) {
	tests := []struct {
		name       string
		runner     *fakeRunner
		repo       *fakeDashboardRepo
		tenantless bool
		wantStatus int
	}{
		{
			name:       "rows_query_failure_returns_500",
			runner:     &fakeRunner{},
			repo:       &fakeDashboardRepo{rowsErr: errors.New("query failed")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "similarity_query_failure_returns_500",
			runner:     &fakeRunner{},
			repo:       &fakeDashboardRepo{simsErr: errors.New("query failed")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "missing_tenant_context_in_pooled_mode_returns_500",
			runner:     &fakeRunner{},
			repo:       &fakeDashboardRepo{},
			tenantless: true,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewDashboardHandler(tc.runner, tc.repo)

			req := dashboardRequest("/api/dashboard")
			if tc.tenantless {
				// A user record without an org yields a context without a
				// tenant — the fake runner then fails like the real one.
				req = requestAs(repository.UserRecord{ID: 1, Role: repository.RoleAdmin}, http.MethodGet, "/api/dashboard", "")
			}
			rec := httptest.NewRecorder()
			h.handleDashboard(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
