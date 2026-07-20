package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrWorkspaceNotFound is returned when no workspace matches the given
// (provider, external_id) pair.
var ErrWorkspaceNotFound = errors.New("platform workspace not found")

// WorkspaceRepo handles persistence for platform_workspaces (Global Directory,
// no RLS): the mapping of external workspace/tenant identifiers to organizations.
type WorkspaceRepo struct {
	pool *pgxpool.Pool
}

// NewWorkspaceRepo constructs a WorkspaceRepo.
func NewWorkspaceRepo(pool *pgxpool.Pool) *WorkspaceRepo {
	return &WorkspaceRepo{pool: pool}
}

// FindOrgByWorkspace resolves an external workspace/tenant identifier to its
// organization ID.
func (r *WorkspaceRepo) FindOrgByWorkspace(ctx context.Context, provider, externalID string) (string, error) {
	const query = `SELECT org_id FROM platform_workspaces WHERE provider = $1 AND external_id = $2`
	var orgID string
	err := r.pool.QueryRow(ctx, query, provider, externalID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrWorkspaceNotFound
	}
	return orgID, err
}

// ClaimWorkspace attempts to register orgID as the owner of the given external
// workspace. If another organization already claimed it (e.g. a concurrent
// first login from the same workspace), the existing owner's org ID is
// returned with created=false so the caller can roll back its candidate org.
func (r *WorkspaceRepo) ClaimWorkspace(ctx context.Context, q Querier, orgID, provider, externalID string) (ownerOrgID string, created bool, err error) {
	const insert = `
		INSERT INTO platform_workspaces (org_id, provider, external_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, external_id) DO NOTHING
		RETURNING org_id`

	err = q.QueryRow(ctx, insert, orgID, provider, externalID).Scan(&ownerOrgID)
	if err == nil {
		return ownerOrgID, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}

	// Conflict: another transaction claimed the workspace first — read the winner.
	const query = `SELECT org_id FROM platform_workspaces WHERE provider = $1 AND external_id = $2`
	err = q.QueryRow(ctx, query, provider, externalID).Scan(&ownerOrgID)
	return ownerOrgID, false, err
}
