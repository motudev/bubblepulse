package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// seedUsers builds the standard cast: an admin (1), a team editor on teamA
// (2), and updaters on teamA (3), teamB (4), and unassigned (5).
func seedUsers() map[int64]repository.UserRecord {
	return map[int64]repository.UserRecord{
		1: {ID: 1, Email: "admin@a.test", Role: repository.RoleAdmin, OrgID: strPtr(orgA)},
		2: {ID: 2, Email: "editor@a.test", Role: repository.RoleTeamEditor, OrgID: strPtr(orgA), TeamID: strPtr(teamA)},
		3: {ID: 3, Email: "member@a.test", Role: repository.RoleUpdater, OrgID: strPtr(orgA), TeamID: strPtr(teamA)},
		4: {ID: 4, Email: "other@a.test", Role: repository.RoleUpdater, OrgID: strPtr(orgA), TeamID: strPtr(teamB)},
		5: {ID: 5, Email: "floating@a.test", Role: repository.RoleUpdater, OrgID: strPtr(orgA)},
	}
}

func updateUser(t *testing.T, users *fakeUsers, teams *fakeTeams, actor repository.UserRecord, targetID, body string) *httptest.ResponseRecorder {
	t.Helper()
	s := newTestServer(&fakeRunner{}, &fakeSessions{}, users, teams, &fakeOrgs{})
	req := requestAs(actor, http.MethodPatch, "/api/v1/users/x", body)
	req.SetPathValue("id", targetID)
	rec := httptest.NewRecorder()
	s.handleUpdateUser(rec, req)
	return rec
}

// visibleTeams seeds the fake with the teams of the actor's org (orgA). teamB
// is deliberately absent in cross-tenant cases.
func visibleTeams(ids ...string) *fakeTeams {
	m := make(map[string]repository.TeamRecord, len(ids))
	for _, id := range ids {
		m[id] = repository.TeamRecord{ID: id, OrgID: orgA, Name: "team " + id[:8]}
	}
	return &fakeTeams{teams: m}
}

// TestHandleUpdateUser_InputValidation covers the request-shape partitions
// that must be rejected before any repository access.
func TestHandleUpdateUser_InputValidation(t *testing.T) {
	admin := seedUsers()[1]

	tests := []struct {
		name       string
		actor      repository.UserRecord
		targetID   string
		body       string
		wantStatus int
	}{
		{"non_numeric_user_id_returns_400", admin, "abc", `{"role": "ADMIN"}`, http.StatusBadRequest},
		{"malformed_json_returns_400", admin, "3", `{"role": `, http.StatusBadRequest},
		{"empty_update_returns_400", admin, "3", `{}`, http.StatusBadRequest},
		{"invalid_role_value_returns_400", admin, "3", `{"role": "SUPERUSER"}`, http.StatusBadRequest},
		{"lowercase_role_value_returns_400", admin, "3", `{"role": "admin"}`, http.StatusBadRequest},
		{"malformed_team_uuid_returns_400", admin, "3", `{"team_id": "not-a-uuid"}`, http.StatusBadRequest},
		{"role_change_by_team_editor_returns_403", seedUsers()[2], "3", `{"role": "UPDATER"}`, http.StatusForbidden},
		{"unknown_target_user_returns_404", admin, "999", `{"role": "UPDATER"}`, http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := &fakeUsers{users: seedUsers()}
			rec := updateUser(t, users, visibleTeams(teamA, teamB), tc.actor, tc.targetID, tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if len(users.setTeamCalls) != 0 || len(users.setRoleCalls) != 0 {
				t.Fatalf("no mutation may happen on rejected input: setTeam=%+v setRole=%+v",
					users.setTeamCalls, users.setRoleCalls)
			}
		})
	}
}

// TestHandleUpdateUser_TeamEditorDecisionTable enumerates the full decision
// table for TEAM_EDITOR team moves (actor's team × target's current team ×
// requested team). Exactly two combinations are permitted: removing a member
// of the editor's own team, and adding an unassigned user to it.
func TestHandleUpdateUser_TeamEditorDecisionTable(t *testing.T) {
	editor := seedUsers()[2] // TEAM_EDITOR on teamA

	tests := []struct {
		name       string
		actor      repository.UserRecord
		targetID   string // 3 = on teamA, 4 = on teamB, 5 = unassigned
		body       string
		wantStatus int
	}{
		// The two permitted rows.
		{"remove_own_team_member_returns_200", editor, "3", `{"team_id": null}`, http.StatusOK},
		{"add_unassigned_user_to_own_team_returns_200", editor, "5", `{"team_id": "` + teamA + `"}`, http.StatusOK},

		// Every other combination is denied.
		{"remove_already_unassigned_user_returns_403", editor, "5", `{"team_id": null}`, http.StatusForbidden},
		{"remove_member_of_foreign_team_returns_403", editor, "4", `{"team_id": null}`, http.StatusForbidden},
		{"poach_member_of_foreign_team_returns_403", editor, "4", `{"team_id": "` + teamA + `"}`, http.StatusForbidden},
		{"move_own_member_to_foreign_team_returns_403", editor, "3", `{"team_id": "` + teamB + `"}`, http.StatusForbidden},
		{"assign_unassigned_user_to_foreign_team_returns_403", editor, "5", `{"team_id": "` + teamB + `"}`, http.StatusForbidden},
		{"reassign_own_member_to_own_team_returns_403", editor, "3", `{"team_id": "` + teamA + `"}`, http.StatusForbidden},
		{
			name:       "editor_without_a_team_is_always_denied_returns_403",
			actor:      repository.UserRecord{ID: 2, Email: "editor@a.test", Role: repository.RoleTeamEditor, OrgID: strPtr(orgA)},
			targetID:   "5",
			body:       `{"team_id": "` + teamA + `"}`,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := &fakeUsers{users: seedUsers()}
			rec := updateUser(t, users, visibleTeams(teamA, teamB), tc.actor, tc.targetID, tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden && len(users.setTeamCalls) != 0 {
				t.Fatalf("denied team change must not mutate, got %+v", users.setTeamCalls)
			}
			if tc.wantStatus == http.StatusOK && len(users.setTeamCalls) != 1 {
				t.Fatalf("permitted team change must call SetTeam exactly once, got %+v", users.setTeamCalls)
			}
		})
	}
}

// TestHandleUpdateUser_AdminTeamChanges covers ADMIN team assignment,
// including the cross-tenant defense: a team invisible in the actor's tenant
// (RLS) must yield 404, never a link into another organization.
func TestHandleUpdateUser_AdminTeamChanges(t *testing.T) {
	admin := seedUsers()[1]

	tests := []struct {
		name       string
		teams      *fakeTeams
		targetID   string
		body       string
		wantStatus int
		wantTeamID *string
	}{
		{
			name:       "admin_assigns_user_to_visible_team_returns_200",
			teams:      visibleTeams(teamA, teamB),
			targetID:   "5",
			body:       `{"team_id": "` + teamB + `"}`,
			wantStatus: http.StatusOK,
			wantTeamID: strPtr(teamB),
		},
		{
			name:       "admin_unassigns_any_user_returns_200",
			teams:      visibleTeams(teamA, teamB),
			targetID:   "4",
			body:       `{"team_id": null}`,
			wantStatus: http.StatusOK,
			wantTeamID: nil,
		},
		{
			name: "cross_tenant_team_returns_404_and_no_mutation",
			// teamB is not visible in this tenant — models another org's team
			// UUID passing format validation but failing the RLS lookup.
			teams:      visibleTeams(teamA),
			targetID:   "5",
			body:       `{"team_id": "` + teamB + `"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := &fakeUsers{users: seedUsers()}
			rec := updateUser(t, users, tc.teams, admin, tc.targetID, tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				if len(users.setTeamCalls) != 0 {
					t.Fatalf("failed team change must not mutate, got %+v", users.setTeamCalls)
				}
				return
			}
			if len(users.setTeamCalls) != 1 {
				t.Fatalf("SetTeam calls = %d, want 1", len(users.setTeamCalls))
			}
			got := users.setTeamCalls[0].teamID
			switch {
			case tc.wantTeamID == nil && got != nil:
				t.Fatalf("SetTeam called with %q, want nil (unassign)", *got)
			case tc.wantTeamID != nil && (got == nil || *got != *tc.wantTeamID):
				t.Fatalf("SetTeam called with %v, want %q", got, *tc.wantTeamID)
			}
		})
	}
}

// TestHandleUpdateUser_RoleChanges covers the role-change rules with the
// last-admin boundary: demoting is allowed at 2 admins and blocked at 1.
func TestHandleUpdateUser_RoleChanges(t *testing.T) {
	tests := []struct {
		name         string
		users        map[int64]repository.UserRecord
		targetID     string
		body         string
		wantStatus   int
		wantSetRoles int
	}{
		{
			name:         "promote_updater_to_editor_returns_200",
			users:        seedUsers(),
			targetID:     "3",
			body:         `{"role": "TEAM_EDITOR"}`,
			wantStatus:   http.StatusOK,
			wantSetRoles: 1,
		},
		{
			name:         "same_role_is_a_no_op_returns_200",
			users:        seedUsers(),
			targetID:     "3",
			body:         `{"role": "UPDATER"}`,
			wantStatus:   http.StatusOK,
			wantSetRoles: 0,
		},
		{
			name:         "demoting_sole_admin_returns_409",
			users:        seedUsers(), // exactly one ADMIN (user 1)
			targetID:     "1",
			body:         `{"role": "UPDATER"}`,
			wantStatus:   http.StatusConflict,
			wantSetRoles: 0,
		},
		{
			name: "demoting_one_of_two_admins_returns_200",
			users: func() map[int64]repository.UserRecord {
				u := seedUsers()
				second := u[3]
				second.Role = repository.RoleAdmin
				u[3] = second
				return u
			}(),
			targetID:     "1",
			body:         `{"role": "UPDATER"}`,
			wantStatus:   http.StatusOK,
			wantSetRoles: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actor := tc.users[1]
			users := &fakeUsers{users: tc.users}
			rec := updateUser(t, users, visibleTeams(teamA, teamB), actor, tc.targetID, tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if len(users.setRoleCalls) != tc.wantSetRoles {
				t.Fatalf("SetRole calls = %d, want %d", len(users.setRoleCalls), tc.wantSetRoles)
			}
			if tc.wantStatus == http.StatusOK && tc.wantSetRoles == 1 {
				var resp orgUserEntry
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("response is not valid JSON: %v", err)
				}
				if resp.Role != mustRole(tc.body) {
					t.Fatalf("response role = %q, want %q", resp.Role, mustRole(tc.body))
				}
			}
		})
	}
}

// mustRole extracts the role field from a request body used in the table above.
func mustRole(body string) string {
	var req struct {
		Role string `json:"role"`
	}
	_ = json.Unmarshal([]byte(body), &req)
	return req.Role
}

func TestHandleListUsers(t *testing.T) {
	tests := []struct {
		name       string
		users      *fakeUsers
		wantStatus int
		wantCount  int
	}{
		{"lists_all_visible_users", &fakeUsers{users: seedUsers()}, http.StatusOK, 5},
		{"empty_org_returns_empty_array", &fakeUsers{}, http.StatusOK, 0},
		{"repository_failure_returns_500", &fakeUsers{listErr: errors.New("list failed")}, http.StatusInternalServerError, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, tc.users, &fakeTeams{}, &fakeOrgs{})
			rec := httptest.NewRecorder()
			s.handleListUsers(rec, requestAs(seedUsers()[1], http.MethodGet, "/api/v1/users", ""))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var entries []orgUserEntry
			if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if len(entries) != tc.wantCount {
				t.Fatalf("entries = %d, want %d", len(entries), tc.wantCount)
			}
		})
	}
}
