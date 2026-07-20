//go:build integration

// Package testhelpers provides shared setup for database integration tests:
// connection to TEST_DATABASE_URL, programmatic goose migrations, table resets
// between tests, and seed helpers that write through the production tenancy
// path so the RLS WITH CHECK clauses are exercised.
package testhelpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// EnvVar is the environment variable holding the test database DSN.
const EnvVar = "TEST_DATABASE_URL"

var (
	migrateOnce sync.Once
	migrateErr  error
)

// Env bundles a migrated test database connection with a pooled-mode tenancy
// runner and the repositories under test. Every Setup call starts from
// truncated tables.
type Env struct {
	DSN        string
	Pool       *pgxpool.Pool
	Runner     *tenancy.Runner // pooled mode: RLS enforced
	Users      *repository.UserRepo
	Teams      *repository.TeamRepo
	Orgs       *repository.OrgRepo
	Sessions   *repository.SessionRepo
	Workspaces *repository.WorkspaceRepo
	Updates    *repository.DailyUpdateRepo
}

// Setup connects to TEST_DATABASE_URL (skipping the test when unset), verifies
// the role cannot bypass RLS, applies migrations once per process, truncates
// all application tables, and returns a ready Env.
func Setup(t *testing.T) *Env {
	t.Helper()

	dsn := os.Getenv(EnvVar)
	if dsn == "" {
		t.Skipf("%s not set; skipping integration test", EnvVar)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// A superuser or BYPASSRLS role would make every isolation assertion pass
	// vacuously — fail loudly instead of testing nothing.
	if err := tenancy.VerifyPooledSafety(ctx, pool); err != nil {
		t.Fatalf("test database role is unsafe for RLS tests: %v", err)
	}

	migrateOnce.Do(func() { migrateErr = migrate(pool) })
	if migrateErr != nil {
		t.Fatalf("apply migrations: %v", migrateErr)
	}

	reset(t, pool)

	return &Env{
		DSN:        dsn,
		Pool:       pool,
		Runner:     tenancy.NewRunner(pool, false),
		Users:      repository.NewUserRepo(pool),
		Teams:      repository.NewTeamRepo(pool),
		Orgs:       repository.NewOrgRepo(pool),
		Sessions:   repository.NewSessionRepo(pool),
		Workspaces: repository.NewWorkspaceRepo(pool),
		Updates:    repository.NewDailyUpdateRepo(pool),
	}
}

// migrate applies the goose migrations from internal/db/migrations, located
// relative to this source file so tests work from any working directory.
func migrate(pool *pgxpool.Pool) error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("cannot determine migrations directory")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "migrations")

	db := stdlib.OpenDBFromPool(pool)
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up (%s): %w", dir, err)
	}
	return nil
}

// reset truncates every application table. TRUNCATE is not subject to row
// security, so this works without a tenant context. River's tables are left
// alone — no worker runs during integration tests.
func reset(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const q = `TRUNCATE TABLE
		daily_update_topics, daily_updates, sessions, user_identities,
		users, teams, platform_workspaces, organizations
		RESTART IDENTITY CASCADE`
	if _, err := pool.Exec(context.Background(), q); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
}

// SiloRunner returns a runner in siloed mode (RLS bypass active) on the same pool.
func (e *Env) SiloRunner() *tenancy.Runner {
	return tenancy.NewRunner(e.Pool, true)
}

// SingleConnPool opens a separate pool limited to one connection, so a query
// after RunTx is guaranteed to reuse the very connection the transaction ran
// on — needed to prove the tenant GUC does not leak across transactions.
func (e *Env) SingleConnPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(e.DSN)
	if err != nil {
		t.Fatalf("parse test DSN: %v", err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open single-connection pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Tenant returns a context bound to the given organization, as the API
// middleware, message service, and workers would produce it.
func Tenant(orgID string) context.Context {
	return tenancy.WithTenantID(context.Background(), orgID)
}

// CreateOrg inserts an organization through the Global Directory (no tenant
// context required) and returns its ID.
func (e *Env) CreateOrg(t *testing.T, name string) string {
	t.Helper()
	id, err := e.Orgs.CreateOrg(context.Background(), e.Pool, name)
	if err != nil {
		t.Fatalf("seed organization %q: %v", name, err)
	}
	return id
}

// CreateTeam inserts a team through a tenant-scoped transaction and returns its ID.
func (e *Env) CreateTeam(t *testing.T, orgID, name string) string {
	t.Helper()
	var id string
	err := e.Runner.RunTx(Tenant(orgID), func(tx pgx.Tx) error {
		var txErr error
		id, txErr = e.Teams.CreateTeam(context.Background(), tx, orgID, name)
		return txErr
	})
	if err != nil {
		t.Fatalf("seed team %q in org %s: %v", name, orgID, err)
	}
	return id
}

// CreateUser upserts a user through a tenant-scoped transaction, optionally
// assigning a team, and returns the user ID.
func (e *Env) CreateUser(t *testing.T, orgID, email, role string, teamID *string) int64 {
	t.Helper()
	var id int64
	err := e.Runner.RunTx(Tenant(orgID), func(tx pgx.Tx) error {
		var txErr error
		id, txErr = e.Users.UpsertUser(context.Background(), tx, orgID, email, email, role)
		if txErr != nil {
			return txErr
		}
		if teamID != nil {
			return e.Users.SetTeam(context.Background(), tx, id, teamID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed user %q in org %s: %v", email, orgID, err)
	}
	return id
}

// CreateUpdate inserts a daily update (created now, so it counts as "today")
// through a tenant-scoped transaction and returns its ID.
func (e *Env) CreateUpdate(t *testing.T, orgID string, userID int64, text string) int64 {
	t.Helper()
	var id int64
	err := e.Runner.RunTx(Tenant(orgID), func(tx pgx.Tx) error {
		var txErr error
		id, txErr = e.Updates.InsertTx(context.Background(), tx, orgID, userID, text)
		return txErr
	})
	if err != nil {
		t.Fatalf("seed daily update for user %d in org %s: %v", userID, orgID, err)
	}
	return id
}

// CreateTopics attaches topics with the given embeddings to a daily update.
func (e *Env) CreateTopics(t *testing.T, orgID string, updateID int64, topics []repository.TopicInsert) {
	t.Helper()
	err := e.Runner.RunTx(Tenant(orgID), func(tx pgx.Tx) error {
		return e.Updates.InsertTopics(context.Background(), tx, orgID, updateID, topics)
	})
	if err != nil {
		t.Fatalf("seed topics for update %d in org %s: %v", updateID, orgID, err)
	}
}

// UnitEmbedding returns a 384-dim one-hot vector with a 1.0 at the given axis
// — orthogonal axes give cosine similarity 0, identical axes give 1.
func UnitEmbedding(axis int) []float32 {
	v := make([]float32, 384)
	v[axis%384] = 1
	return v
}
