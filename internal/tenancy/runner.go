package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoTenant is returned by RunTx in pooled mode when the context carries no tenant ID.
var ErrNoTenant = errors.New("tenancy: no tenant in context in pooled mode")

// Runner opens tenant-scoped database transactions.
//
// In pooled mode it executes SET LOCAL app.current_tenant_id (via set_config
// with is_local=true, the parameterized equivalent — plain SET LOCAL cannot
// take bind parameters) so the RLS policies apply. In siloed mode it sets
// app.is_siloed instead, activating the bypass condition baked into every
// policy. Because the setting is transaction-local, Postgres discards it on
// commit or rollback before the connection returns to the pool, so pooled
// connections can never leak a tenant context.
type Runner struct {
	pool   *pgxpool.Pool
	siloed bool
}

// NewRunner constructs a Runner. siloed selects single-tenant silo mode.
func NewRunner(pool *pgxpool.Pool, siloed bool) *Runner {
	return &Runner{pool: pool, siloed: siloed}
}

// Siloed reports whether the runner operates in single-tenant silo mode.
func (r *Runner) Siloed() bool { return r.siloed }

// RunTx begins a transaction, binds it to the tenant carried by ctx (or to
// silo mode), invokes fn, and commits. Any error rolls the transaction back.
func (r *Runner) RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if r.siloed {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.is_siloed', 'true', true)`); err != nil {
			return fmt.Errorf("set silo mode: %w", err)
		}
	} else {
		orgID, ok := TenantIDFromContext(ctx)
		if !ok {
			return ErrNoTenant
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant_id', $1, true)`, orgID); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
