package api

// Hand-written test doubles for the consumer interfaces defined in server.go
// and dashboard.go. The fake runner mirrors tenancy.Runner's contract —
// including ErrNoTenant in pooled mode — so handlers are exercised against
// the same failure modes as production. Fakes invoke handler callbacks with a
// nil pgx.Tx; the store fakes never touch their Querier argument.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// Stable UUIDs for two tenants and their teams.
const (
	orgA  = "11111111-1111-1111-1111-111111111111"
	orgB  = "22222222-2222-2222-2222-222222222222"
	teamA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	teamB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func strPtr(s string) *string { return &s }

type fakeRunner struct {
	siloed bool
	err    error // returned instead of invoking fn
}

func (f *fakeRunner) RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	if f.err != nil {
		return f.err
	}
	if !f.siloed {
		if _, ok := tenancy.TenantIDFromContext(ctx); !ok {
			return tenancy.ErrNoTenant
		}
	}
	return fn(nil)
}

func (f *fakeRunner) Siloed() bool { return f.siloed }

type fakeSessions struct {
	sessions map[string]repository.SessionRecord
	err      error // overrides lookup when set
}

func (f *fakeSessions) FindByToken(ctx context.Context, token string) (repository.SessionRecord, error) {
	if f.err != nil {
		return repository.SessionRecord{}, f.err
	}
	rec, ok := f.sessions[token]
	if !ok {
		return repository.SessionRecord{}, repository.ErrSessionNotFound
	}
	return rec, nil
}

type setTeamCall struct {
	userID int64
	teamID *string
}

type setRoleCall struct {
	userID int64
	role   string
}

type fakeUsers struct {
	users   map[int64]repository.UserRecord
	findErr error
	listErr error

	setTeamCalls []setTeamCall
	setRoleCalls []setRoleCall
}

func (f *fakeUsers) FindByID(ctx context.Context, q repository.Querier, id int64) (repository.UserRecord, error) {
	if f.findErr != nil {
		return repository.UserRecord{}, f.findErr
	}
	u, ok := f.users[id]
	if !ok {
		return repository.UserRecord{}, pgx.ErrNoRows
	}
	return u, nil
}

func (f *fakeUsers) ListByOrg(ctx context.Context, q repository.Querier) ([]repository.UserRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []repository.UserRecord
	for _, u := range f.users {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeUsers) SetTeam(ctx context.Context, q repository.Querier, userID int64, teamID *string) error {
	f.setTeamCalls = append(f.setTeamCalls, setTeamCall{userID: userID, teamID: teamID})
	u, ok := f.users[userID]
	if !ok {
		return pgx.ErrNoRows
	}
	u.TeamID = teamID
	f.users[userID] = u
	return nil
}

func (f *fakeUsers) SetRole(ctx context.Context, q repository.Querier, userID int64, role string) error {
	f.setRoleCalls = append(f.setRoleCalls, setRoleCall{userID: userID, role: role})
	u, ok := f.users[userID]
	if !ok {
		return pgx.ErrNoRows
	}
	u.Role = role
	f.users[userID] = u
	return nil
}

func (f *fakeUsers) CountAdmins(ctx context.Context, q repository.Querier) (int, error) {
	n := 0
	for _, u := range f.users {
		if u.Role == repository.RoleAdmin {
			n++
		}
	}
	return n, nil
}

type renameTeamCall struct {
	teamID string
	name   string
}

type createTeamCall struct {
	orgID string
	name  string
}

type fakeTeams struct {
	teams     map[string]repository.TeamRecord
	createID  string
	createErr error
	listErr   error

	createCalls []createTeamCall
	renameCalls []renameTeamCall
	deleteCalls []string
}

func (f *fakeTeams) ListTeams(ctx context.Context, q repository.Querier) ([]repository.TeamRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []repository.TeamRecord
	for _, t := range f.teams {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeTeams) FindTeamByID(ctx context.Context, q repository.Querier, id string) (repository.TeamRecord, error) {
	t, ok := f.teams[id]
	if !ok {
		return repository.TeamRecord{}, repository.ErrTeamNotFound
	}
	return t, nil
}

func (f *fakeTeams) CreateTeam(ctx context.Context, q repository.Querier, orgID, name string) (string, error) {
	f.createCalls = append(f.createCalls, createTeamCall{orgID: orgID, name: name})
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createID, nil
}

func (f *fakeTeams) RenameTeam(ctx context.Context, q repository.Querier, teamID, name string) error {
	f.renameCalls = append(f.renameCalls, renameTeamCall{teamID: teamID, name: name})
	if _, ok := f.teams[teamID]; !ok {
		return repository.ErrTeamNotFound
	}
	return nil
}

func (f *fakeTeams) DeleteTeam(ctx context.Context, q repository.Querier, teamID string) error {
	f.deleteCalls = append(f.deleteCalls, teamID)
	if _, ok := f.teams[teamID]; !ok {
		return repository.ErrTeamNotFound
	}
	delete(f.teams, teamID)
	return nil
}

type renameOrgCall struct {
	orgID string
	name  string
}

type fakeOrgs struct {
	orgs      map[string]repository.OrgRecord
	renameErr error

	renameCalls []renameOrgCall
}

func (f *fakeOrgs) FindOrgByID(ctx context.Context, id string) (repository.OrgRecord, error) {
	o, ok := f.orgs[id]
	if !ok {
		return repository.OrgRecord{}, repository.ErrOrgNotFound
	}
	return o, nil
}

func (f *fakeOrgs) RenameOrg(ctx context.Context, id, name string) error {
	f.renameCalls = append(f.renameCalls, renameOrgCall{orgID: id, name: name})
	if f.renameErr != nil {
		return f.renameErr
	}
	if _, ok := f.orgs[id]; !ok {
		return repository.ErrOrgNotFound
	}
	return nil
}

type fakeDashboardRepo struct {
	rows    []repository.DashboardRowWithTopics
	sims    []repository.TopicSimilarityRow
	rowsErr error
	simsErr error

	gotTeamIDs []*string // one entry per FindLatestPerUserWithTopics call
}

func (f *fakeDashboardRepo) FindLatestPerUserWithTopics(ctx context.Context, q repository.Querier, teamID *string) ([]repository.DashboardRowWithTopics, error) {
	f.gotTeamIDs = append(f.gotTeamIDs, teamID)
	if f.rowsErr != nil {
		return nil, f.rowsErr
	}
	return f.rows, nil
}

func (f *fakeDashboardRepo) FindTodayTopicSimilarities(ctx context.Context, q repository.Querier, teamID *string) ([]repository.TopicSimilarityRow, error) {
	if f.simsErr != nil {
		return nil, f.simsErr
	}
	return f.sims, nil
}

// newTestServer wires a Server directly from fakes, bypassing New so no OIDC
// handler or platform adapters are needed.
func newTestServer(runner tenantTxRunner, sessions SessionLookup, users UserStore, teams TeamStore, orgs OrgStore) *Server {
	return &Server{
		mux:      http.NewServeMux(),
		runner:   runner,
		sessions: sessions,
		users:    users,
		teams:    teams,
		orgs:     orgs,
	}
}

// requestAs builds a request carrying the context that requireSession and
// requireRole would have injected for the given user: user ID, current-user
// record, and (when the user has an org) the tenant ID.
func requestAs(user repository.UserRecord, method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), userIDKey, user.ID)
	ctx = context.WithValue(ctx, currentUserKey, user)
	if user.OrgID != nil {
		ctx = tenancy.WithTenantID(ctx, *user.OrgID)
	}
	return req.WithContext(ctx)
}
