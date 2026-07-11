// Package repository provides pgx-backed implementations of domain repository interfaces.
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo implements auth.UserRepository using a pgx connection pool.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo constructs a UserRepo.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// UserRecord holds the user fields returned by FindByID.
type UserRecord struct {
	ID    int64
	Email string
	Name  string
}

// UpsertUser inserts or updates a user by email and returns their ID.
func (r *UserRepo) UpsertUser(ctx context.Context, email, name string) (int64, error) {
	const q = `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE
		  SET name = EXCLUDED.name, updated_at = NOW()
		RETURNING id`

	var id int64
	if err := r.pool.QueryRow(ctx, q, email, name).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// FindByID returns a user by primary key.
func (r *UserRepo) FindByID(ctx context.Context, id int64) (UserRecord, error) {
	const q = `SELECT id, email, name FROM users WHERE id = $1`
	var u UserRecord
	err := r.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.Name)
	return u, err
}

// UpsertIdentity links a provider identity to a user; no-op if the identity already exists.
func (r *UserRepo) UpsertIdentity(ctx context.Context, userID int64, provider, providerID string) error {
	const q = `
		INSERT INTO user_identities (user_id, provider, provider_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, provider_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, q, userID, provider, providerID)
	return err
}
