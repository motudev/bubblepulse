package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

func adminIn(org string) repository.UserRecord {
	return repository.UserRecord{ID: 1, Email: "admin@a.test", Role: repository.RoleAdmin, OrgID: strPtr(org)}
}

func editorOf(org, team string) repository.UserRecord {
	return repository.UserRecord{ID: 2, Email: "editor@a.test", Role: repository.RoleTeamEditor, OrgID: strPtr(org), TeamID: strPtr(team)}
}

func TestHandleListTeams(t *testing.T) {
	tests := []struct {
		name       string
		teams      *fakeTeams
		wantStatus int
		wantCount  int
	}{
		{
			name: "lists_visible_teams",
			teams: &fakeTeams{teams: map[string]repository.TeamRecord{
				teamA: {ID: teamA, OrgID: orgA, Name: "Platform"},
			}},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "empty_org_returns_empty_array",
			teams:      &fakeTeams{},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "repository_failure_returns_500",
			teams:      &fakeTeams{listErr: errors.New("list failed")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, &fakeUsers{}, tc.teams, &fakeOrgs{})
			rec := httptest.NewRecorder()
			s.handleListTeams(rec, requestAs(adminIn(orgA), http.MethodGet, "/api/v1/teams", ""))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var entries []teamEntry
			if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if len(entries) != tc.wantCount {
				t.Fatalf("entries = %d, want %d", len(entries), tc.wantCount)
			}
		})
	}
}

func TestHandleCreateTeam(t *testing.T) {
	tests := []struct {
		name       string
		actor      repository.UserRecord
		body       string
		createErr  error
		wantStatus int
		wantName   string // asserted against the repo call on 201
	}{
		{
			name:       "admin_creates_team_in_own_org_returns_201",
			actor:      adminIn(orgA),
			body:       `{"name": "Platform"}`,
			wantStatus: http.StatusCreated,
			wantName:   "Platform",
		},
		{
			name:       "name_is_trimmed_before_persisting",
			actor:      adminIn(orgA),
			body:       `{"name": "  Platform  "}`,
			wantStatus: http.StatusCreated,
			wantName:   "Platform",
		},
		{
			name:       "empty_name_returns_400",
			actor:      adminIn(orgA),
			body:       `{"name": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "whitespace_only_name_returns_400",
			actor:      adminIn(orgA),
			body:       `{"name": "   "}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed_json_returns_400",
			actor:      adminIn(orgA),
			body:       `{"name": `,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "actor_without_org_returns_403",
			actor:      repository.UserRecord{ID: 1, Role: repository.RoleAdmin},
			body:       `{"name": "Platform"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "repository_failure_returns_500",
			actor:      adminIn(orgA),
			body:       `{"name": "Platform"}`,
			createErr:  errors.New("insert failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			teams := &fakeTeams{createID: teamA, createErr: tc.createErr}
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, &fakeUsers{}, teams, &fakeOrgs{})

			rec := httptest.NewRecorder()
			s.handleCreateTeam(rec, requestAs(tc.actor, http.MethodPost, "/api/v1/teams", tc.body))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusCreated {
				if tc.createErr == nil && len(teams.createCalls) != 0 {
					t.Fatalf("repository must not be called on rejected input, got %+v", teams.createCalls)
				}
				return
			}
			if len(teams.createCalls) != 1 {
				t.Fatalf("CreateTeam calls = %d, want 1", len(teams.createCalls))
			}
			call := teams.createCalls[0]
			if call.orgID != *tc.actor.OrgID {
				t.Fatalf("team created in org %q, want the actor's org %q", call.orgID, *tc.actor.OrgID)
			}
			if call.name != tc.wantName {
				t.Fatalf("team name persisted = %q, want %q", call.name, tc.wantName)
			}
			var resp teamEntry
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if resp.ID != teamA || resp.Name != tc.wantName {
				t.Fatalf("response = %+v, want id %q name %q", resp, teamA, tc.wantName)
			}
		})
	}
}

func TestHandleUpdateTeam(t *testing.T) {
	tests := []struct {
		name       string
		actor      repository.UserRecord
		teamID     string
		body       string
		wantStatus int
	}{
		{
			name:       "admin_renames_any_team_returns_200",
			actor:      adminIn(orgA),
			teamID:     teamA,
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "team_editor_renames_own_team_returns_200",
			actor:      editorOf(orgA, teamA),
			teamID:     teamA,
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "team_editor_renaming_foreign_team_returns_403",
			actor:      editorOf(orgA, teamA),
			teamID:     teamB,
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "team_editor_without_team_returns_403",
			actor:      repository.UserRecord{ID: 2, Role: repository.RoleTeamEditor, OrgID: strPtr(orgA)},
			teamID:     teamA,
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid_uuid_returns_400",
			actor:      adminIn(orgA),
			teamID:     "not-a-uuid",
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "uuid_one_char_short_returns_400",
			actor:      adminIn(orgA),
			teamID:     teamA[:35],
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty_name_returns_400",
			actor:      adminIn(orgA),
			teamID:     teamA,
			body:       `{"name": "  "}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown_team_returns_404",
			actor:      adminIn(orgA),
			teamID:     teamB, // not seeded below — models a cross-tenant or deleted team
			body:       `{"name": "Renamed"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			teams := &fakeTeams{teams: map[string]repository.TeamRecord{
				teamA: {ID: teamA, OrgID: orgA, Name: "Old"},
			}}
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, &fakeUsers{}, teams, &fakeOrgs{})

			req := requestAs(tc.actor, http.MethodPatch, "/api/v1/teams/"+tc.teamID, tc.body)
			req.SetPathValue("id", tc.teamID)
			rec := httptest.NewRecorder()
			s.handleUpdateTeam(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden || tc.wantStatus == http.StatusBadRequest {
				if len(teams.renameCalls) != 0 {
					t.Fatalf("repository must not be called on rejected input, got %+v", teams.renameCalls)
				}
			}
		})
	}
}

func TestHandleDeleteTeam(t *testing.T) {
	tests := []struct {
		name       string
		teamID     string
		wantStatus int
	}{
		{"existing_team_returns_204", teamA, http.StatusNoContent},
		{"unknown_team_returns_404", teamB, http.StatusNotFound},
		{"invalid_uuid_returns_400", "'; DROP TABLE teams; --", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			teams := &fakeTeams{teams: map[string]repository.TeamRecord{
				teamA: {ID: teamA, OrgID: orgA, Name: "Platform"},
			}}
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, &fakeUsers{}, teams, &fakeOrgs{})

			// The handler reads the ID from the path value, so the raw URL can
			// stay fixed — important for the injection-shaped case, which is
			// not a valid URL path.
			req := requestAs(adminIn(orgA), http.MethodDelete, "/api/v1/teams/x", "")
			req.SetPathValue("id", tc.teamID)
			rec := httptest.NewRecorder()
			s.handleDeleteTeam(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusBadRequest && len(teams.deleteCalls) != 0 {
				t.Fatalf("repository must not be called on invalid input, got %+v", teams.deleteCalls)
			}
		})
	}
}
