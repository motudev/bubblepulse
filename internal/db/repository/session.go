package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionRepo implements auth.SessionRepository using a pgx connection pool.
type SessionRepo struct {
	pool *pgxpool.Pool
}

// NewSessionRepo constructs a SessionRepo.
func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// Create inserts a new session token for the given user.
func (r *SessionRepo) Create(ctx context.Context, userID int64, token string) error {
	const q = `INSERT INTO sessions (user_id, token) VALUES ($1, $2)`
	_, err := r.pool.Exec(ctx, q, userID, token)
	return err
}
