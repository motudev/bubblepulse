package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionNotFound is returned when no valid session matches the given token.
var ErrSessionNotFound = errors.New("session not found or expired")

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

// FindUserIDByToken returns the user ID for a valid, non-expired session token.
func (r *SessionRepo) FindUserIDByToken(ctx context.Context, token string) (int64, error) {
	const q = `SELECT user_id FROM sessions WHERE token = $1 AND expires_at > NOW()`
	var userID int64
	err := r.pool.QueryRow(ctx, q, token).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSessionNotFound
	}
	return userID, err
}

// Delete removes a session by its token. Idempotent — no error if the token is absent.
func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	_, err := r.pool.Exec(ctx, q, token)
	return err
}
