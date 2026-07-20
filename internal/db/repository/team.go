package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTeamNotFound is returned when no team in the current tenant context
// matches the given ID.
var ErrTeamNotFound = errors.New("team not found")

// TeamRepo handles persistence for teams. The table is RLS-protected:
// every method requires a tenant-scoped Querier.
type TeamRepo struct {
	pool *pgxpool.Pool
}

// NewTeamRepo constructs a TeamRepo.
func NewTeamRepo(pool *pgxpool.Pool) *TeamRepo {
	return &TeamRepo{pool: pool}
}

// TeamRecord holds the team fields returned by read methods.
type TeamRecord struct {
	ID    string
	OrgID string
	Name  string
}

// ListTeams returns all teams visible in the current tenant context, ordered by name.
func (r *TeamRepo) ListTeams(ctx context.Context, q Querier) ([]TeamRecord, error) {
	const query = `SELECT id, org_id, name FROM teams ORDER BY name, id`
	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []TeamRecord
	for rows.Next() {
		var t TeamRecord
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// FindTeamByID returns a team by primary key. Because RLS scopes the lookup,
// this doubles as the visibility check that a team belongs to the caller's
// organization (foreign-key validation alone bypasses RLS).
func (r *TeamRepo) FindTeamByID(ctx context.Context, q Querier, id string) (TeamRecord, error) {
	const query = `SELECT id, org_id, name FROM teams WHERE id = $1`
	var t TeamRecord
	err := q.QueryRow(ctx, query, id).Scan(&t.ID, &t.OrgID, &t.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return TeamRecord{}, ErrTeamNotFound
	}
	return t, err
}

// CreateTeam inserts a new team for the given organization and returns its
// generated UUID. org_id must match the tenant context (RLS WITH CHECK).
func (r *TeamRepo) CreateTeam(ctx context.Context, q Querier, orgID, name string) (string, error) {
	const query = `INSERT INTO teams (org_id, name) VALUES ($1, $2) RETURNING id`
	var id string
	err := q.QueryRow(ctx, query, orgID, name).Scan(&id)
	return id, err
}

// RenameTeam updates a team's name.
func (r *TeamRepo) RenameTeam(ctx context.Context, q Querier, teamID, name string) error {
	const query = `UPDATE teams SET name = $1 WHERE id = $2`
	tag, err := q.Exec(ctx, query, name, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

// DeleteTeam removes a team; members fall back to no team via fk_users_team
// ON DELETE SET NULL.
func (r *TeamRepo) DeleteTeam(ctx context.Context, q Querier, teamID string) error {
	const query = `DELETE FROM teams WHERE id = $1`
	tag, err := q.Exec(ctx, query, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}
