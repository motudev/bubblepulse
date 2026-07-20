//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/db/testhelpers"
)

// TestUpsertUser_EmailUniquePerOrg pins the pooled-tenancy identity model:
// the same email address is a distinct user in every organization
// (uq_users_org_email), and re-upserting within one org updates in place
// without touching the established role.
func TestUpsertUser_EmailUniquePerOrg(t *testing.T) {
	env := testhelpers.Setup(t)
	ctx := context.Background()
	orgA := env.CreateOrg(t, "Org A")
	orgB := env.CreateOrg(t, "Org B")

	idInA := env.CreateUser(t, orgA, "same@person.test", repository.RoleAdmin, nil)
	idInB := env.CreateUser(t, orgB, "same@person.test", repository.RoleUpdater, nil)

	if idInA == idInB {
		t.Fatalf("same email in two orgs collapsed into one user (id %d)", idInA)
	}

	t.Run("each_org_sees_only_its_own_user", func(t *testing.T) {
		err := env.Runner.RunTx(testhelpers.Tenant(orgA), func(tx pgx.Tx) error {
			users, err := env.Users.ListByOrg(ctx, tx)
			if err != nil {
				return err
			}
			if len(users) != 1 || users[0].ID != idInA {
				t.Fatalf("org A sees %+v, want only its own user %d", users, idInA)
			}
			if _, err := env.Users.FindByID(ctx, tx, idInB); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("org A can read org B's user: %v", err)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("tenant transaction failed: %v", err)
		}
	})

	t.Run("reupsert_updates_name_but_keeps_role", func(t *testing.T) {
		err := env.Runner.RunTx(testhelpers.Tenant(orgA), func(tx pgx.Tx) error {
			// A later login passes RoleUpdater as roleIfNew; the existing
			// ADMIN must not be demoted by it.
			again, err := env.Users.UpsertUser(ctx, tx, orgA, "same@person.test", "New Name", repository.RoleUpdater)
			if err != nil {
				return err
			}
			if again != idInA {
				t.Fatalf("re-upsert created a second user %d, want existing %d", again, idInA)
			}
			u, err := env.Users.FindByID(ctx, tx, idInA)
			if err != nil {
				return err
			}
			if u.Name != "New Name" {
				t.Fatalf("name = %q, want updated %q", u.Name, "New Name")
			}
			if u.Role != repository.RoleAdmin {
				t.Fatalf("role = %q; roleIfNew must not overwrite an existing role", u.Role)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("tenant transaction failed: %v", err)
		}
	})
}

func TestIdentityResolution(t *testing.T) {
	env := testhelpers.Setup(t)
	ctx := context.Background()
	orgA := env.CreateOrg(t, "Org A")
	orgB := env.CreateOrg(t, "Org B")
	userID := env.CreateUser(t, orgA, "alice@a.test", repository.RoleAdmin, nil)

	t.Run("unknown_identity_is_not_found", func(t *testing.T) {
		_, err := env.Users.FindIdentity(ctx, slackProvider, "U_UNKNOWN")
		if !errors.Is(err, repository.ErrIdentityNotFound) {
			t.Fatalf("FindIdentity = %v, want ErrIdentityNotFound", err)
		}
	})

	t.Run("upsert_then_find_resolves_user_and_org", func(t *testing.T) {
		if err := env.Users.UpsertIdentity(ctx, userID, slackProvider, "U123", orgA); err != nil {
			t.Fatalf("UpsertIdentity: %v", err)
		}
		rec, err := env.Users.FindIdentity(ctx, slackProvider, "U123")
		if err != nil {
			t.Fatalf("FindIdentity: %v", err)
		}
		if rec.UserID != userID || rec.OrgID == nil || *rec.OrgID != orgA {
			t.Fatalf("identity = %+v, want user %d in org %s", rec, userID, orgA)
		}
	})

	t.Run("legacy_identity_gets_org_backfilled", func(t *testing.T) {
		// A pre-tenancy row has org_id NULL; the next upsert must fill it.
		_, err := env.Pool.Exec(ctx,
			`INSERT INTO user_identities (user_id, provider, provider_id, org_id) VALUES ($1, $2, $3, NULL)`,
			userID, slackProvider, "U_LEGACY")
		if err != nil {
			t.Fatalf("seed legacy identity: %v", err)
		}
		if err := env.Users.UpsertIdentity(ctx, userID, slackProvider, "U_LEGACY", orgA); err != nil {
			t.Fatalf("UpsertIdentity backfill: %v", err)
		}
		rec, err := env.Users.FindIdentity(ctx, slackProvider, "U_LEGACY")
		if err != nil {
			t.Fatalf("FindIdentity: %v", err)
		}
		if rec.OrgID == nil || *rec.OrgID != orgA {
			t.Fatalf("org not backfilled: %+v", rec)
		}
	})

	t.Run("established_identity_org_is_immutable", func(t *testing.T) {
		// An upsert claiming a different org must not move the identity —
		// COALESCE keeps the first binding.
		if err := env.Users.UpsertIdentity(ctx, userID, slackProvider, "U123", orgB); err != nil {
			t.Fatalf("UpsertIdentity with foreign org: %v", err)
		}
		rec, err := env.Users.FindIdentity(ctx, slackProvider, "U123")
		if err != nil {
			t.Fatalf("FindIdentity: %v", err)
		}
		if rec.OrgID == nil || *rec.OrgID != orgA {
			t.Fatalf("identity org changed to %v, must stay %s", rec.OrgID, orgA)
		}
	})
}

func TestSessionLifecycle(t *testing.T) {
	env := testhelpers.Setup(t)
	ctx := context.Background()
	orgA := env.CreateOrg(t, "Org A")
	userID := env.CreateUser(t, orgA, "alice@a.test", repository.RoleAdmin, nil)

	t.Run("create_then_find_returns_user_and_org", func(t *testing.T) {
		if err := env.Sessions.Create(ctx, userID, "tok-valid", orgA); err != nil {
			t.Fatalf("Create: %v", err)
		}
		rec, err := env.Sessions.FindByToken(ctx, "tok-valid")
		if err != nil {
			t.Fatalf("FindByToken: %v", err)
		}
		if rec.UserID != userID || rec.OrgID == nil || *rec.OrgID != orgA {
			t.Fatalf("session = %+v, want user %d org %s", rec, userID, orgA)
		}
	})

	t.Run("unknown_token_is_not_found", func(t *testing.T) {
		if _, err := env.Sessions.FindByToken(ctx, "tok-missing"); !errors.Is(err, repository.ErrSessionNotFound) {
			t.Fatalf("FindByToken = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("expired_token_is_not_found", func(t *testing.T) {
		_, err := env.Pool.Exec(ctx,
			`INSERT INTO sessions (user_id, token, org_id, expires_at) VALUES ($1, $2, $3, NOW() - INTERVAL '1 minute')`,
			userID, "tok-expired", orgA)
		if err != nil {
			t.Fatalf("seed expired session: %v", err)
		}
		if _, err := env.Sessions.FindByToken(ctx, "tok-expired"); !errors.Is(err, repository.ErrSessionNotFound) {
			t.Fatalf("FindByToken(expired) = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("delete_invalidates_and_is_idempotent", func(t *testing.T) {
		if err := env.Sessions.Create(ctx, userID, "tok-delete", orgA); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := env.Sessions.Delete(ctx, "tok-delete"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := env.Sessions.FindByToken(ctx, "tok-delete"); !errors.Is(err, repository.ErrSessionNotFound) {
			t.Fatalf("deleted session still resolves: %v", err)
		}
		if err := env.Sessions.Delete(ctx, "tok-delete"); err != nil {
			t.Fatalf("second Delete must be idempotent, got %v", err)
		}
	})
}
