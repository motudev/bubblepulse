package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
)

// --- test doubles ---

type mockOrgs struct {
	createOrgFn func(ctx context.Context, q repository.Querier, name string) (string, error)
}

func (m *mockOrgs) CreateOrg(ctx context.Context, q repository.Querier, name string) (string, error) {
	return m.createOrgFn(ctx, q, name)
}

type mockWorkspaces struct {
	claimFn       func(ctx context.Context, q repository.Querier, orgID, provider, externalID string) (string, bool, error)
	upsertTokenFn func(ctx context.Context, provider, externalID, teamName, botToken string) error
}

func (m *mockWorkspaces) ClaimWorkspace(ctx context.Context, q repository.Querier, orgID, provider, externalID string) (string, bool, error) {
	return m.claimFn(ctx, q, orgID, provider, externalID)
}

func (m *mockWorkspaces) UpsertBotToken(ctx context.Context, provider, externalID, teamName, botToken string) error {
	return m.upsertTokenFn(ctx, provider, externalID, teamName, botToken)
}

// --- handleInstall ---

func TestHandleInstall_SetsStateCookieAndRedirects(t *testing.T) {
	inst := &Installer{
		clientID:    "my-client",
		redirectURL: "http://localhost/api/slack/callback",
		secure:      false,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/slack/install", nil)
	rr := httptest.NewRecorder()

	inst.handleInstall(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rr.Code)
	}

	loc := rr.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("invalid redirect location: %v", err)
	}
	if parsed.Host != "slack.com" {
		t.Errorf("expected redirect to slack.com, got %q", parsed.Host)
	}
	if parsed.Query().Get("state") == "" {
		t.Error("state query param must not be empty")
	}
	if parsed.Query().Get("client_id") != "my-client" {
		t.Errorf("expected client_id=my-client, got %q", parsed.Query().Get("client_id"))
	}

	var cookieFound bool
	for _, c := range rr.Result().Cookies() {
		if c.Name != installStateCookieName {
			continue
		}
		cookieFound = true
		if c.Value == "" {
			t.Error("state cookie value must not be empty")
		}
		if !c.HttpOnly {
			t.Error("state cookie must be HttpOnly")
		}
		if c.Path != "/api/slack/callback" {
			t.Errorf("state cookie path: want /api/slack/callback, got %q", c.Path)
		}
		if c.Value != parsed.Query().Get("state") {
			t.Error("cookie state must match redirect state param")
		}
	}
	if !cookieFound {
		t.Errorf("%s cookie not set", installStateCookieName)
	}
}

// --- handleCallback CSRF guard ---

func TestHandleCallback_StateMismatch(t *testing.T) {
	tests := []struct {
		name        string
		cookieValue string
		stateParam  string
	}{
		{name: "no cookie", cookieValue: "", stateParam: "abc"},
		{name: "wrong cookie", cookieValue: "xxx", stateParam: "abc"},
		{name: "empty state param", cookieValue: "abc", stateParam: ""},
	}

	inst := &Installer{secure: false}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/slack/callback?state="+tc.stateParam+"&code=c", nil)
			if tc.cookieValue != "" {
				req.AddCookie(&http.Cookie{Name: installStateCookieName, Value: tc.cookieValue})
			}
			rr := httptest.NewRecorder()
			inst.handleCallback(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rr.Code)
			}
		})
	}
}

// --- exchangeCode ---

func TestExchangeCode(t *testing.T) {
	tests := []struct {
		name        string
		slackBody   map[string]any
		wantErr     bool
		wantToken   string
		wantTeamID  string
	}{
		{
			name: "happy path",
			slackBody: map[string]any{
				"ok":           true,
				"access_token": "xoxb-test-token",
				"team":         map[string]string{"id": "T123", "name": "Acme"},
				"authed_user":  map[string]string{"id": "U001"},
			},
			wantToken:  "xoxb-test-token",
			wantTeamID: "T123",
		},
		{
			name:      "slack returns ok=false",
			slackBody: map[string]any{"ok": false, "error": "invalid_code"},
			wantErr:   true,
		},
		{
			name:      "empty access_token",
			slackBody: map[string]any{"ok": true, "access_token": "", "team": map[string]string{"id": "T1", "name": "X"}},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("want POST, got %s", r.Method)
				}
				if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
					t.Errorf("unexpected Content-Type: %s", ct)
				}
				if r.Header.Get("Authorization") == "" {
					t.Error("expected Basic auth header")
				}
				_ = r.ParseForm()
				if r.FormValue("code") != "test-code" {
					t.Errorf("expected code=test-code, got %q", r.FormValue("code"))
				}
				_ = json.NewEncoder(w).Encode(tc.slackBody)
			}))
			defer srv.Close()

			inst := &Installer{
				clientID:     "cid",
				clientSecret: "csec",
				redirectURL:  "https://example.com/callback",
			}

			// Redirect outbound calls to the test server.
			orig := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: &rewriteTransport{target: srv.URL}}
			defer func() { http.DefaultClient = orig }()

			result, err := inst.exchangeCode(context.Background(), "test-code")
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.AccessToken != tc.wantToken {
				t.Errorf("token: want %q, got %q", tc.wantToken, result.AccessToken)
			}
			if result.Team.ID != tc.wantTeamID {
				t.Errorf("team.id: want %q, got %q", tc.wantTeamID, result.Team.ID)
			}
		})
	}
}

// rewriteTransport redirects all outbound requests to a fixed base URL.
type rewriteTransport struct{ target string }

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	parsed, _ := url.Parse(rt.target)
	cloned.URL.Scheme = parsed.Scheme
	cloned.URL.Host = parsed.Host
	cloned.URL.Path = ""
	return http.DefaultTransport.RoundTrip(cloned)
}
