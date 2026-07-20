package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

func TestHandleUpdateOrg(t *testing.T) {
	tests := []struct {
		name       string
		actor      repository.UserRecord
		body       string
		renameErr  error
		wantStatus int
		wantName   string // asserted against the repo call on 200
	}{
		{
			name:       "admin_renames_own_org_returns_200",
			actor:      adminIn(orgA),
			body:       `{"name": "Acme"}`,
			wantStatus: http.StatusOK,
			wantName:   "Acme",
		},
		{
			name:       "name_is_trimmed_before_persisting",
			actor:      adminIn(orgA),
			body:       `{"name": "  Acme  "}`,
			wantStatus: http.StatusOK,
			wantName:   "Acme",
		},
		{
			name:       "empty_name_returns_400",
			actor:      adminIn(orgA),
			body:       `{"name": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed_json_returns_400",
			actor:      adminIn(orgA),
			body:       `{"name"`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "actor_without_org_returns_403",
			actor:      repository.UserRecord{ID: 1, Role: repository.RoleAdmin},
			body:       `{"name": "Acme"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "org_missing_in_directory_returns_404",
			actor:      adminIn(orgA),
			body:       `{"name": "Acme"}`,
			renameErr:  repository.ErrOrgNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "repository_failure_returns_500",
			actor:      adminIn(orgA),
			body:       `{"name": "Acme"}`,
			renameErr:  errors.New("update failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orgs := &fakeOrgs{
				orgs:      map[string]repository.OrgRecord{orgA: {ID: orgA, Name: ""}},
				renameErr: tc.renameErr,
			}
			s := newTestServer(&fakeRunner{}, &fakeSessions{}, &fakeUsers{}, &fakeTeams{}, orgs)

			rec := httptest.NewRecorder()
			s.handleUpdateOrg(rec, requestAs(tc.actor, http.MethodPatch, "/api/v1/org", tc.body))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest || tc.wantStatus == http.StatusForbidden {
				if len(orgs.renameCalls) != 0 {
					t.Fatalf("repository must not be called on rejected input, got %+v", orgs.renameCalls)
				}
				return
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			if len(orgs.renameCalls) != 1 {
				t.Fatalf("RenameOrg calls = %d, want 1", len(orgs.renameCalls))
			}
			call := orgs.renameCalls[0]
			// The renamed org must always be the actor's own — the request
			// body carries no org ID that could redirect the write.
			if call.orgID != *tc.actor.OrgID {
				t.Fatalf("renamed org %q, want the actor's org %q", call.orgID, *tc.actor.OrgID)
			}
			if call.name != tc.wantName {
				t.Fatalf("persisted name = %q, want %q", call.name, tc.wantName)
			}
			var resp orgInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if resp.ID != orgA || resp.Name != tc.wantName {
				t.Fatalf("response = %+v, want id %q name %q", resp, orgA, tc.wantName)
			}
		})
	}
}
