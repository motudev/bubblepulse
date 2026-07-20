package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionNotFound is returned when no valid session matches the given token.
var ErrSessionNotFound = errors.New("session not found or expired")

// SessionRepo handles persistence for sessions (Global Directory, no RLS —
// the session row is how the API middleware learns the tenant before opening
// an RLS-scoped transaction).
type SessionRepo struct {
	pool *pgxpool.Pool
}

// NewSessionRepo constructs a SessionRepo.
func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// SessionRecord holds the resolution of a session token.
type SessionRecord struct {
	UserID int64
	OrgID  *string
}

// Create inserts a new session token for the given user and organization.
func (r *SessionRepo) Create(ctx context.Context, userID int64, token, orgID string) error {
	const q = `INSERT INTO sessions (user_id, token, org_id) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, q, userID, token, orgID)
	return err
}

// FindByToken returns the user and organization for a valid, non-expired
// session token. OrgID is nil only for legacy pre-tenancy sessions.
func (r *SessionRepo) FindByToken(ctx context.Context, token string) (SessionRecord, error) {
	const q = `SELECT user_id, org_id FROM sessions WHERE token = $1 AND expires_at > NOW()`
	var rec SessionRecord
	err := r.pool.QueryRow(ctx, q, token).Scan(&rec.UserID, &rec.OrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrSessionNotFound
	}
	return rec, err
}

// Delete removes a session by its token. Idempotent — no error if the token is absent.
func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	_, err := r.pool.Exec(ctx, q, token)
	return err
}
