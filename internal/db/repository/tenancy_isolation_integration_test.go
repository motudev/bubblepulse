//go:build integration

package repository_test

// The tenant-isolation acceptance suite: exercises the real RLS policies and
// the tenancy.Runner GUC binding against a live Postgres (TEST_DATABASE_URL),
// connected as a non-BYPASSRLS role. Every case seeds two organizations and
// proves org A can neither see nor mutate org B's rows.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/db/testhelpers"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// twoOrgs seeds the standard fixture: org A with two users and a team, org B
// with one user and a team.
type twoOrgs struct {
	orgA, orgB   string
	teamA, teamB string
	adminA       int64
	memberA      int64
	userB        int64
}

func seedTwoOrgs(t *testing.T, env *testhelpers.Env) twoOrgs {
	t.Helper()
	f := twoOrgs{}
	f.orgA = env.CreateOrg(t, "Org A")
	f.orgB = env.CreateOrg(t, "Org B")
	f.teamA = env.CreateTeam(t, f.orgA, "Team A")
	f.teamB = env.CreateTeam(t, f.orgB, "Team B")
	f.adminA = env.CreateUser(t, f.orgA, "admin@a.test", repository.RoleAdmin, nil)
	f.memberA = env.CreateUser(t, f.orgA, "member@a.test", repository.RoleUpdater, &f.teamA)
	f.userB = env.CreateUser(t, f.orgB, "user@b.test", repository.RoleAdmin, &f.teamB)
	return f
}

func TestTenantIsolation_Reads(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedTwoOrgs(t, env)
	ctx := context.Background()

	err := env.Runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
		t.Run("list_users_returns_only_own_org", func(t *testing.T) {
			users, err := env.Users.ListByOrg(ctx, tx)
			if err != nil {
				t.Fatalf("ListByOrg: %v", err)
			}
			if len(users) != 2 {
				t.Fatalf("visible users = %d, want 2 (org A only)", len(users))
			}
			for _, u := range users {
				if u.OrgID == nil || *u.OrgID != f.orgA {
					t.Fatalf("user %q leaked from another org: %+v", u.Email, u)
				}
			}
		})

		t.Run("list_teams_returns_only_own_org", func(t *testing.T) {
			teams, err := env.Teams.ListTeams(ctx, tx)
			if err != nil {
				t.Fatalf("ListTeams: %v", err)
			}
			if len(teams) != 1 || teams[0].ID != f.teamA {
				t.Fatalf("visible teams = %+v, want only team A", teams)
			}
		})

		t.Run("foreign_user_by_id_is_not_found", func(t *testing.T) {
			_, err := env.Users.FindByID(ctx, tx, f.userB)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("FindByID(org B user) = %v, want pgx.ErrNoRows", err)
			}
		})

		t.Run("foreign_team_by_id_is_not_found", func(t *testing.T) {
			_, err := env.Teams.FindTeamByID(ctx, tx, f.teamB)
			if !errors.Is(err, repository.ErrTeamNotFound) {
				t.Fatalf("FindTeamByID(org B team) = %v, want ErrTeamNotFound", err)
			}
		})

		t.Run("count_admins_counts_only_own_org", func(t *testing.T) {
			// Org B also has an ADMIN; it must not inflate org A's count.
			n, err := env.Users.CountAdmins(ctx, tx)
			if err != nil {
				t.Fatalf("CountAdmins: %v", err)
			}
			if n != 1 {
				t.Fatalf("CountAdmins = %d, want 1", n)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("tenant transaction failed: %v", err)
	}
}

func TestTenantIsolation_CrossTenantWritesAreNoOps(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedTwoOrgs(t, env)
	ctx := context.Background()

	// Every mutation attempt runs under org A's tenant context against org
	// B's rows and must hit zero rows.
	err := env.Runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
		t.Run("set_team_on_foreign_user_is_not_found", func(t *testing.T) {
			if err := env.Users.SetTeam(ctx, tx, f.userB, nil); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("SetTeam(org B user) = %v, want pgx.ErrNoRows", err)
			}
		})
		t.Run("set_role_on_foreign_user_is_not_found", func(t *testing.T) {
			if err := env.Users.SetRole(ctx, tx, f.userB, repository.RoleUpdater); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("SetRole(org B user) = %v, want pgx.ErrNoRows", err)
			}
		})
		t.Run("rename_foreign_team_is_not_found", func(t *testing.T) {
			if err := env.Teams.RenameTeam(ctx, tx, f.teamB, "hijacked"); !errors.Is(err, repository.ErrTeamNotFound) {
				t.Fatalf("RenameTeam(org B team) = %v, want ErrTeamNotFound", err)
			}
		})
		t.Run("delete_foreign_team_is_not_found", func(t *testing.T) {
			if err := env.Teams.DeleteTeam(ctx, tx, f.teamB); !errors.Is(err, repository.ErrTeamNotFound) {
				t.Fatalf("DeleteTeam(org B team) = %v, want ErrTeamNotFound", err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("tenant transaction failed: %v", err)
	}

	// Prove org B's state is untouched, reading as org B.
	err = env.Runner.RunTx(testhelpers.Tenant(f.orgB), func(tx pgx.Tx) error {
		u, err := env.Users.FindByID(ctx, tx, f.userB)
		if err != nil {
			t.Fatalf("re-read org B user: %v", err)
		}
		if u.Role != repository.RoleAdmin || u.TeamID == nil || *u.TeamID != f.teamB {
			t.Fatalf("org B user was mutated cross-tenant: %+v", u)
		}
		team, err := env.Teams.FindTeamByID(ctx, tx, f.teamB)
		if err != nil {
			t.Fatalf("re-read org B team: %v", err)
		}
		if team.Name != "Team B" {
			t.Fatalf("org B team was renamed cross-tenant: %+v", team)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verification transaction failed: %v", err)
	}
}

func TestTenantIsolation_InsertIntoForeignOrgViolatesWithCheck(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedTwoOrgs(t, env)
	ctx := context.Background()

	tests := []struct {
		name   string
		insert func(tx pgx.Tx) error
	}{
		{
			name: "create_team_for_foreign_org_is_rejected",
			insert: func(tx pgx.Tx) error {
				_, err := env.Teams.CreateTeam(ctx, tx, f.orgB, "smuggled")
				return err
			},
		},
		{
			name: "upsert_user_into_foreign_org_is_rejected",
			insert: func(tx pgx.Tx) error {
				_, err := env.Users.UpsertUser(ctx, tx, f.orgB, "smuggled@b.test", "x", repository.RoleUpdater)
				return err
			},
		},
		{
			name: "insert_update_for_foreign_org_is_rejected",
			insert: func(tx pgx.Tx) error {
				_, err := env.Updates.InsertTx(ctx, tx, f.orgB, f.userB, "smuggled")
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Bound to org A, writing rows claiming to be org B's: the RLS
			// WITH CHECK clause must reject each insert.
			err := env.Runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
				return tc.insert(tx)
			})
			if err == nil {
				t.Fatal("insert into a foreign org must fail, got nil error")
			}
		})
	}
}

func TestTenantIsolation_FailClosedWithoutTenantContext(t *testing.T) {
	env := testhelpers.Setup(t)
	seedTwoOrgs(t, env)
	ctx := context.Background()

	// Querying straight from the pool binds no tenant GUC: the fail-closed
	// policy must hide every row of every RLS table, despite seeded data.
	for _, table := range []string{"users", "teams", "daily_updates", "daily_update_topics"} {
		t.Run("unbound_connection_sees_empty_"+table, func(t *testing.T) {
			var n int
			if err := env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			if n != 0 {
				t.Fatalf("%s visible without tenant context: %d rows leaked", table, n)
			}
		})
	}
}

func TestRunner_PooledModeWithoutTenantReturnsErrNoTenant(t *testing.T) {
	env := testhelpers.Setup(t)

	called := false
	err := env.Runner.RunTx(context.Background(), func(tx pgx.Tx) error {
		called = true
		return nil
	})
	if !errors.Is(err, tenancy.ErrNoTenant) {
		t.Fatalf("RunTx without tenant = %v, want ErrNoTenant", err)
	}
	if called {
		t.Fatal("transaction callback must not run without a tenant")
	}
}

func TestRunner_SiloedModeSeesAllOrganizations(t *testing.T) {
	env := testhelpers.Setup(t)
	seedTwoOrgs(t, env)
	ctx := context.Background()

	err := env.SiloRunner().RunTx(context.Background(), func(tx pgx.Tx) error {
		users, err := env.Users.ListByOrg(ctx, tx)
		if err != nil {
			t.Fatalf("ListByOrg in silo mode: %v", err)
		}
		if len(users) != 3 {
			t.Fatalf("silo mode sees %d users, want all 3 across both orgs", len(users))
		}
		teams, err := env.Teams.ListTeams(ctx, tx)
		if err != nil {
			t.Fatalf("ListTeams in silo mode: %v", err)
		}
		if len(teams) != 2 {
			t.Fatalf("silo mode sees %d teams, want 2", len(teams))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("silo transaction failed: %v", err)
	}
}

func TestRunner_TenantGUCIsTransactionLocal(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedTwoOrgs(t, env)
	ctx := context.Background()

	// A single-connection pool guarantees the post-transaction query reuses
	// the exact connection the tenant transaction ran on.
	pool := env.SingleConnPool(t)
	runner := tenancy.NewRunner(pool, false)

	err := runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
		var setting string
		if err := tx.QueryRow(ctx, `SELECT current_setting('app.current_tenant_id', true)`).Scan(&setting); err != nil {
			return err
		}
		if setting != f.orgA {
			t.Fatalf("GUC inside transaction = %q, want %q", setting, f.orgA)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tenant transaction failed: %v", err)
	}

	var after *string
	if err := pool.QueryRow(ctx, `SELECT NULLIF(current_setting('app.current_tenant_id', true), '')`).Scan(&after); err != nil {
		t.Fatalf("read GUC after commit: %v", err)
	}
	if after != nil {
		t.Fatalf("tenant GUC leaked across transaction boundary: %q", *after)
	}

	// And the connection must again see zero RLS rows.
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users after commit: %v", err)
	}
	if n != 0 {
		t.Fatalf("connection still sees %d users after the tenant transaction ended", n)
	}
}
