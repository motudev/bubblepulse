package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DailyUpdateRepo handles persistence for daily_updates.
type DailyUpdateRepo struct {
	pool *pgxpool.Pool
}

// NewDailyUpdateRepo constructs a DailyUpdateRepo.
func NewDailyUpdateRepo(pool *pgxpool.Pool) *DailyUpdateRepo {
	return &DailyUpdateRepo{pool: pool}
}

// DashboardRow holds user profile and their most recent update for the dashboard.
type DashboardRow struct {
	UserID     int64
	Name       string
	Email      string
	UpdateText *string
	UpdateAt   *time.Time
}

// Insert adds a new update row for the given user.
func (r *DailyUpdateRepo) Insert(ctx context.Context, userID int64, text string) error {
	const q = `INSERT INTO daily_updates (user_id, update_text) VALUES ($1, $2)`
	_, err := r.pool.Exec(ctx, q, userID, text)
	return err
}

// FindLatestPerUser returns the single most recent active (deleted_at IS NULL) update
// for every user in the system. Users with no updates are still included with nil fields.
func (r *DailyUpdateRepo) FindLatestPerUser(ctx context.Context) ([]DashboardRow, error) {
	const q = `
		SELECT DISTINCT ON (u.id)
			u.id, u.name, u.email,
			du.update_text, du.created_at
		FROM users u
		LEFT JOIN daily_updates du ON du.user_id = u.id AND du.deleted_at IS NULL
		ORDER BY u.id, du.created_at DESC NULLS LAST`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DashboardRow
	for rows.Next() {
		var row DashboardRow
		if err := rows.Scan(&row.UserID, &row.Name, &row.Email, &row.UpdateText, &row.UpdateAt); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
