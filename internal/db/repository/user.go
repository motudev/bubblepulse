// Package repository provides pgx-backed implementations of domain repository interfaces.
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrIdentityNotFound is returned when no identity matches the given (provider, provider_id) pair.
var ErrIdentityNotFound = errors.New("identity not found")

// UserRepo handles persistence for users (RLS-protected) and
// user_identities (Global Directory, no RLS).
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo constructs a UserRepo.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// UserRecord holds the user fields returned by read methods.
type UserRecord struct {
	ID     int64
	Email  string
	Name   string
	Role   string
	OrgID  *string
	TeamID *string
}

// IdentityRecord holds the Global Directory resolution of an external identity.
type IdentityRecord struct {
	UserID int64
	OrgID  *string
}

// UpsertUser inserts or updates a user scoped to an organization and returns
// their ID. roleIfNew applies only when the row is created; existing users
// keep their current role. Runs on an RLS table — requires a tenant-scoped Querier.
func (r *UserRepo) UpsertUser(ctx context.Context, q Querier, orgID, email, name, roleIfNew string) (int64, error) {
	const query = `
		INSERT INTO users (org_id, email, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, email) DO UPDATE
		  SET name = EXCLUDED.name, updated_at = NOW()
		RETURNING id`

	var id int64
	if err := q.QueryRow(ctx, query, orgID, email, name, roleIfNew).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// FindByID returns a user by primary key. Requires a tenant-scoped Querier.
func (r *UserRepo) FindByID(ctx context.Context, q Querier, id int64) (UserRecord, error) {
	const query = `SELECT id, email, name, role, org_id, team_id FROM users WHERE id = $1`
	var u UserRecord
	err := q.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.OrgID, &u.TeamID)
	return u, err
}

// ListByOrg returns all users visible in the current tenant context,
// ordered by name. Requires a tenant-scoped Querier.
func (r *UserRepo) ListByOrg(ctx context.Context, q Querier) ([]UserRecord, error) {
	const query = `SELECT id, email, name, role, org_id, team_id FROM users ORDER BY name, id`
	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.OrgID, &u.TeamID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetTeam assigns (or, with nil, clears) a user's team. Requires a tenant-scoped Querier.
func (r *UserRepo) SetTeam(ctx context.Context, q Querier, userID int64, teamID *string) error {
	const query = `UPDATE users SET team_id = $1, updated_at = NOW() WHERE id = $2`
	tag, err := q.Exec(ctx, query, teamID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetRole changes a user's role. Requires a tenant-scoped Querier.
func (r *UserRepo) SetRole(ctx context.Context, q Querier, userID int64, role string) error {
	const query = `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`
	tag, err := q.Exec(ctx, query, role, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CountAdmins returns the number of ADMIN users visible in the current tenant
// context. Used to protect against demoting the last admin of an organization.
func (r *UserRepo) CountAdmins(ctx context.Context, q Querier) (int, error) {
	const query = `SELECT COUNT(*) FROM users WHERE role = $1`
	var n int
	err := q.QueryRow(ctx, query, RoleAdmin).Scan(&n)
	return n, err
}

// FindIdentity resolves an external identity to the internal user and their
// organization. Global Directory query — runs pre-tenant on the pool.
func (r *UserRepo) FindIdentity(ctx context.Context, provider, providerID string) (IdentityRecord, error) {
	const query = `SELECT user_id, org_id FROM user_identities WHERE provider = $1 AND provider_id = $2`
	var rec IdentityRecord
	err := r.pool.QueryRow(ctx, query, provider, providerID).Scan(&rec.UserID, &rec.OrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdentityRecord{}, ErrIdentityNotFound
	}
	return rec, err
}

// UpsertIdentity links a provider identity to a user and its organization.
// Existing rows have their org_id backfilled if it is still NULL (legacy rows
// created before multi-tenancy). Global Directory query — runs on the pool.
func (r *UserRepo) UpsertIdentity(ctx context.Context, userID int64, provider, providerID, orgID string) error {
	const query = `
		INSERT INTO user_identities (user_id, provider, provider_id, org_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_id) DO UPDATE
		  SET org_id = COALESCE(user_identities.org_id, EXCLUDED.org_id)`

	_, err := r.pool.Exec(ctx, query, userID, provider, providerID, orgID)
	return err
}
