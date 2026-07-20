package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRLSBypassRole indicates the connected database role would silently skip
// row-level security, breaking tenant isolation in pooled mode.
var ErrRLSBypassRole = errors.New("tenancy: connected role is superuser or has BYPASSRLS; row-level security would not apply")

// VerifyPooledSafety fails when the connected role bypasses RLS. Superusers
// and BYPASSRLS roles ignore policies even on tables with FORCE ROW LEVEL
// SECURITY, so running pooled mode with such a role would silently expose
// every tenant's rows.
func VerifyPooledSafety(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`
	var bypasses bool
	if err := pool.QueryRow(ctx, q).Scan(&bypasses); err != nil {
		return fmt.Errorf("check role RLS capabilities: %w", err)
	}
	if bypasses {
		return ErrRLSBypassRole
	}
	return nil
}
