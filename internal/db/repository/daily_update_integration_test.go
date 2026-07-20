//go:build integration

package repository_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/db/testhelpers"
)

// updatesFixture seeds two tenants with today's updates and topics:
// org A: adminA without an update, memberA (team A) whose *latest* of two
// updates carries topics "alpha" and "beta" (identical embeddings → sim 1.0).
// org B: userB with an update ("b-secret") and topics on a different axis.
type updatesFixture struct {
	twoOrgs
	latestUpdateA int64
	updateB       int64
}

func seedUpdates(t *testing.T, env *testhelpers.Env) updatesFixture {
	t.Helper()
	f := updatesFixture{twoOrgs: seedTwoOrgs(t, env)}

	env.CreateUpdate(t, f.orgA, f.memberA, "first draft")
	f.latestUpdateA = env.CreateUpdate(t, f.orgA, f.memberA, "second update")
	env.CreateTopics(t, f.orgA, f.latestUpdateA, []repository.TopicInsert{
		{ExtractedTopic: "alpha", Embedding: testhelpers.UnitEmbedding(0)},
		{ExtractedTopic: "beta", Embedding: testhelpers.UnitEmbedding(0)},
	})

	f.updateB = env.CreateUpdate(t, f.orgB, f.userB, "b-secret")
	env.CreateTopics(t, f.orgB, f.updateB, []repository.TopicInsert{
		{ExtractedTopic: "gamma", Embedding: testhelpers.UnitEmbedding(1)},
		{ExtractedTopic: "delta", Embedding: testhelpers.UnitEmbedding(1)},
	})
	return f
}

func TestDashboardQueries_ScopedToTenant(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedUpdates(t, env)
	ctx := context.Background()

	err := env.Runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
		t.Run("latest_per_user_returns_only_own_org", func(t *testing.T) {
			rows, err := env.Updates.FindLatestPerUserWithTopics(ctx, tx, nil)
			if err != nil {
				t.Fatalf("FindLatestPerUserWithTopics: %v", err)
			}
			if len(rows) != 2 {
				t.Fatalf("visible rows = %d, want 2 (org A users only)", len(rows))
			}
			for _, row := range rows {
				if row.UpdateText != nil && *row.UpdateText == "b-secret" {
					t.Fatalf("org B's update leaked into org A's dashboard: %+v", row)
				}
			}
		})

		t.Run("latest_update_wins_over_earlier_one", func(t *testing.T) {
			rows, err := env.Updates.FindLatestPerUserWithTopics(ctx, tx, nil)
			if err != nil {
				t.Fatalf("FindLatestPerUserWithTopics: %v", err)
			}
			var member *repository.DashboardRowWithTopics
			for i := range rows {
				if rows[i].UserID == f.memberA {
					member = &rows[i]
				}
			}
			if member == nil || member.UpdateText == nil {
				t.Fatalf("member A missing or without update: %+v", rows)
			}
			if *member.UpdateText != "second update" {
				t.Fatalf("update text = %q, want the latest (%q)", *member.UpdateText, "second update")
			}
			if len(member.Topics) != 2 {
				t.Fatalf("topics = %v, want [alpha beta]", member.Topics)
			}
		})

		t.Run("user_without_update_appears_with_null_fields", func(t *testing.T) {
			rows, err := env.Updates.FindLatestPerUserWithTopics(ctx, tx, nil)
			if err != nil {
				t.Fatalf("FindLatestPerUserWithTopics: %v", err)
			}
			for _, row := range rows {
				if row.UserID == f.adminA {
					if row.UpdateText != nil || row.UpdateAt != nil {
						t.Fatalf("admin A has no update today, got %+v", row)
					}
					if row.Topics == nil || len(row.Topics) != 0 {
						t.Fatalf("topics must be an empty non-nil slice, got %v", row.Topics)
					}
					return
				}
			}
			t.Fatal("admin A missing from dashboard rows")
		})

		t.Run("own_team_filter_narrows_to_members", func(t *testing.T) {
			teamID := f.teamA
			rows, err := env.Updates.FindLatestPerUserWithTopics(ctx, tx, &teamID)
			if err != nil {
				t.Fatalf("FindLatestPerUserWithTopics(team A): %v", err)
			}
			if len(rows) != 1 || rows[0].UserID != f.memberA {
				t.Fatalf("team-filtered rows = %+v, want only member A", rows)
			}
		})

		t.Run("foreign_team_filter_yields_empty_not_foreign_data", func(t *testing.T) {
			// Passing another org's team UUID (as the dashboard endpoint
			// allows) must return nothing — never org B's members.
			teamID := f.teamB
			rows, err := env.Updates.FindLatestPerUserWithTopics(ctx, tx, &teamID)
			if err != nil {
				t.Fatalf("FindLatestPerUserWithTopics(team B): %v", err)
			}
			if len(rows) != 0 {
				t.Fatalf("foreign team filter returned %d rows, want 0: %+v", len(rows), rows)
			}
		})

		t.Run("foreign_update_text_is_not_found", func(t *testing.T) {
			_, err := env.Updates.FindUpdateTextByID(ctx, tx, f.updateB)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("FindUpdateTextByID(org B update) = %v, want pgx.ErrNoRows", err)
			}
		})

		t.Run("similarities_include_only_own_topics", func(t *testing.T) {
			sims, err := env.Updates.FindTodayTopicSimilarities(ctx, tx, nil)
			if err != nil {
				t.Fatalf("FindTodayTopicSimilarities: %v", err)
			}
			if len(sims) != 1 {
				t.Fatalf("similarity pairs = %+v, want exactly (alpha, beta)", sims)
			}
			pair := sims[0]
			if pair.TopicA != "alpha" || pair.TopicB != "beta" {
				t.Fatalf("pair = (%q, %q), want (alpha, beta)", pair.TopicA, pair.TopicB)
			}
			if math.Abs(pair.Similarity-1.0) > 1e-6 {
				t.Fatalf("similarity of identical embeddings = %v, want 1.0", pair.Similarity)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("tenant transaction failed: %v", err)
	}
}

func TestSetUpdateEmbedding_CrossTenantWriteIsNoOp(t *testing.T) {
	env := testhelpers.Setup(t)
	f := seedUpdates(t, env)
	ctx := context.Background()

	// Bound to org A, targeting org B's update: RLS hits zero rows. The
	// method reports no error (embedding writes are fire-and-forget), so the
	// assertion is that org B's row is provably untouched.
	err := env.Runner.RunTx(testhelpers.Tenant(f.orgA), func(tx pgx.Tx) error {
		return env.Updates.SetUpdateEmbedding(ctx, tx, f.updateB, testhelpers.UnitEmbedding(2))
	})
	if err != nil {
		t.Fatalf("cross-tenant SetUpdateEmbedding must be a silent no-op, got %v", err)
	}

	err = env.Runner.RunTx(testhelpers.Tenant(f.orgB), func(tx pgx.Tx) error {
		var embeddingIsNull bool
		if err := tx.QueryRow(ctx, `SELECT update_embedding IS NULL FROM daily_updates WHERE id = $1`, f.updateB).Scan(&embeddingIsNull); err != nil {
			return err
		}
		if !embeddingIsNull {
			t.Fatal("org B's update embedding was written from org A's tenant context")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verification transaction failed: %v", err)
	}
}
