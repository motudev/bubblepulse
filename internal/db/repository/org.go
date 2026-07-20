package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOrgNotFound is returned when no organization matches the given ID.
var ErrOrgNotFound = errors.New("organization not found")

// OrgRepo handles persistence for organizations (Global Directory, no RLS).
type OrgRepo struct {
	pool *pgxpool.Pool
}

// NewOrgRepo constructs an OrgRepo.
func NewOrgRepo(pool *pgxpool.Pool) *OrgRepo {
	return &OrgRepo{pool: pool}
}

// OrgRecord holds the organization fields returned by read methods.
type OrgRecord struct {
	ID   string
	Name string
}

// CreateOrg inserts a new organization (name may be blank when it cannot be
// inferred from the login provider) and returns its generated UUID. Takes a
// Querier so provisioning can run inside the same transaction that claims the
// platform workspace.
func (r *OrgRepo) CreateOrg(ctx context.Context, q Querier, name string) (string, error) {
	const query = `INSERT INTO organizations (name) VALUES ($1) RETURNING id`
	var id string
	err := q.QueryRow(ctx, query, name).Scan(&id)
	return id, err
}

// FindOrgByID returns an organization by primary key.
func (r *OrgRepo) FindOrgByID(ctx context.Context, id string) (OrgRecord, error) {
	const query = `SELECT id, name FROM organizations WHERE id = $1`
	var org OrgRecord
	err := r.pool.QueryRow(ctx, query, id).Scan(&org.ID, &org.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrgRecord{}, ErrOrgNotFound
	}
	return org, err
}

// RenameOrg updates an organization's name.
func (r *OrgRepo) RenameOrg(ctx context.Context, id, name string) error {
	const query = `UPDATE organizations SET name = $1 WHERE id = $2`
	tag, err := r.pool.Exec(ctx, query, name, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOrgNotFound
	}
	return nil
}
