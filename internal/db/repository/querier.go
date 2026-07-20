package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the subset of pgx operations shared by *pgxpool.Pool and pgx.Tx.
//
// Methods touching RLS-protected tables (users, teams, daily_updates,
// daily_update_topics) take a Querier and must be invoked with a transaction
// opened by tenancy.Runner.RunTx, which binds the tenant GUC the policies
// check. Global Directory tables (organizations, platform_workspaces,
// user_identities, sessions) are queried straight from the pool.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}
