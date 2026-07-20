package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// TestRequireSession walks the session lifecycle as a state-transition table:
// no cookie → invalid token → lookup failure → valid but org-less (pooled vs
// siloed) → valid with org.
func TestRequireSession(t *testing.T) {
	tests := []struct {
		name         string
		cookie       string // empty = no cookie sent
		sessions     *fakeSessions
		siloed       bool
		wantStatus   int
		wantUserID   int64  // asserted only on 200
		wantTenantID string // asserted only on 200; empty = expect no tenant
	}{
		{
			name:       "missing_cookie_returns_401",
			sessions:   &fakeSessions{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown_token_returns_401",
			cookie:     "deadbeef",
			sessions:   &fakeSessions{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "session_lookup_db_error_returns_401",
			cookie:     "deadbeef",
			sessions:   &fakeSessions{err: errors.New("connection refused")},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "orgless_session_in_pooled_mode_returns_401",
			cookie: "legacy",
			sessions: &fakeSessions{sessions: map[string]repository.SessionRecord{
				"legacy": {UserID: 7, OrgID: nil},
			}},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "orgless_session_in_siloed_mode_passes",
			cookie: "legacy",
			sessions: &fakeSessions{sessions: map[string]repository.SessionRecord{
				"legacy": {UserID: 7, OrgID: nil},
			}},
			siloed:     true,
			wantStatus: http.StatusOK,
			wantUserID: 7,
		},
		{
			name:   "valid_session_injects_user_and_tenant",
			cookie: "good",
			sessions: &fakeSessions{sessions: map[string]repository.SessionRecord{
				"good": {UserID: 42, OrgID: strPtr(orgA)},
			}},
			wantStatus:   http.StatusOK,
			wantUserID:   42,
			wantTenantID: orgA,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(&fakeRunner{siloed: tc.siloed}, tc.sessions, &fakeUsers{}, &fakeTeams{}, &fakeOrgs{})

			var gotUserID int64
			var gotUserOK bool
			var gotTenantID string
			next := func(w http.ResponseWriter, r *http.Request) {
				gotUserID, gotUserOK = UserIDFromContext(r.Context())
				gotTenantID, _ = tenancy.TenantIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "session", Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			s.requireSession(next)(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				if gotUserOK {
					t.Fatal("next handler must not run on rejected sessions")
				}
				return
			}
			if !gotUserOK || gotUserID != tc.wantUserID {
				t.Fatalf("user ID in context = %d (ok=%v), want %d", gotUserID, gotUserOK, tc.wantUserID)
			}
			if gotTenantID != tc.wantTenantID {
				t.Fatalf("tenant ID in context = %q, want %q", gotTenantID, tc.wantTenantID)
			}
		})
	}
}

// TestRequireRole partitions the stored role against the route's allowlist
// and covers the failure paths: unknown user, transaction failure.
func TestRequireRole(t *testing.T) {
	const token = "good"
	session := map[string]repository.SessionRecord{
		token: {UserID: 1, OrgID: strPtr(orgA)},
	}
	userWithRole := func(role string) map[int64]repository.UserRecord {
		return map[int64]repository.UserRecord{
			1: {ID: 1, Email: "a@a.test", Role: role, OrgID: strPtr(orgA)},
		}
	}

	tests := []struct {
		name       string
		allowed    []string
		users      *fakeUsers
		runner     *fakeRunner
		wantStatus int
	}{
		{
			name:       "admin_allowed_on_admin_route",
			allowed:    []string{repository.RoleAdmin},
			users:      &fakeUsers{users: userWithRole(repository.RoleAdmin)},
			wantStatus: http.StatusOK,
		},
		{
			name:       "team_editor_allowed_on_admin_or_editor_route",
			allowed:    []string{repository.RoleAdmin, repository.RoleTeamEditor},
			users:      &fakeUsers{users: userWithRole(repository.RoleTeamEditor)},
			wantStatus: http.StatusOK,
		},
		{
			name:       "updater_denied_admin_route_returns_403",
			allowed:    []string{repository.RoleAdmin},
			users:      &fakeUsers{users: userWithRole(repository.RoleUpdater)},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "team_editor_denied_admin_only_route_returns_403",
			allowed:    []string{repository.RoleAdmin},
			users:      &fakeUsers{users: userWithRole(repository.RoleTeamEditor)},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unknown_role_value_denied_returns_403",
			allowed:    []string{repository.RoleAdmin, repository.RoleTeamEditor, repository.RoleUpdater},
			users:      &fakeUsers{users: userWithRole("SUPERUSER")},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "user_row_missing_returns_401",
			allowed:    []string{repository.RoleAdmin},
			users:      &fakeUsers{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "transaction_failure_returns_401",
			allowed:    []string{repository.RoleAdmin},
			users:      &fakeUsers{users: userWithRole(repository.RoleAdmin)},
			runner:     &fakeRunner{err: errors.New("db down")},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := tc.runner
			if runner == nil {
				runner = &fakeRunner{}
			}
			s := newTestServer(runner, &fakeSessions{sessions: session}, tc.users, &fakeTeams{}, &fakeOrgs{})

			var gotUser repository.UserRecord
			var gotUserOK bool
			next := func(w http.ResponseWriter, r *http.Request) {
				gotUser, gotUserOK = CurrentUserFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
			req.AddCookie(&http.Cookie{Name: "session", Value: token})
			rec := httptest.NewRecorder()
			s.requireRole(tc.allowed...)(next)(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				if !gotUserOK || gotUser.ID != 1 {
					t.Fatalf("current user in context = %+v (ok=%v), want user 1", gotUser, gotUserOK)
				}
			} else if gotUserOK {
				t.Fatal("next handler must not run when the role check rejects")
			}
		})
	}
}
