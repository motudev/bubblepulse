package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DailyUpdateRepo handles persistence for daily_updates and daily_update_topics.
// Both tables are RLS-protected: every method requires a tenant-scoped Querier.
type DailyUpdateRepo struct {
	pool *pgxpool.Pool
}

// NewDailyUpdateRepo constructs a DailyUpdateRepo.
func NewDailyUpdateRepo(pool *pgxpool.Pool) *DailyUpdateRepo {
	return &DailyUpdateRepo{pool: pool}
}

// DashboardRowWithTopics holds a user's profile, their most recent update,
// and the update's extracted topics for the dashboard.
type DashboardRowWithTopics struct {
	UserID     int64
	Name       string
	Email      string
	UpdateText *string
	UpdateAt   *time.Time
	Topics     []string
}

// TopicInsert holds a single extracted topic and its embedding for batch insertion.
type TopicInsert struct {
	ExtractedTopic string
	Embedding      []float32
}

// TopicSimilarityRow holds a pairwise cosine similarity between two topic strings.
type TopicSimilarityRow struct {
	TopicA     string
	TopicB     string
	Similarity float64
}

// InsertTx adds a new update row within a caller-managed tenant transaction
// and returns the generated ID.
func (r *DailyUpdateRepo) InsertTx(ctx context.Context, tx pgx.Tx, orgID string, userID int64, text string) (int64, error) {
	const q = `INSERT INTO daily_updates (org_id, user_id, update_text) VALUES ($1, $2, $3) RETURNING id`
	var id int64
	err := tx.QueryRow(ctx, q, orgID, userID, text).Scan(&id)
	return id, err
}

// FindUpdateTextByID returns the update_text for the given daily_update ID.
func (r *DailyUpdateRepo) FindUpdateTextByID(ctx context.Context, q Querier, id int64) (string, error) {
	const query = `SELECT update_text FROM daily_updates WHERE id = $1`
	var text string
	err := q.QueryRow(ctx, query, id).Scan(&text)
	return text, err
}

// SetUpdateEmbedding stores the 384-dim embedding vector for a daily update row.
func (r *DailyUpdateRepo) SetUpdateEmbedding(ctx context.Context, q Querier, id int64, embedding []float32) error {
	const query = `UPDATE daily_updates SET update_embedding = $1::vector WHERE id = $2`
	_, err := q.Exec(ctx, query, floatSliceToVectorLiteral(embedding), id)
	return err
}

// InsertTopics batch-inserts extracted topic rows for a given daily update.
// Each row carries org_id explicitly to satisfy the RLS WITH CHECK clause.
func (r *DailyUpdateRepo) InsertTopics(ctx context.Context, q Querier, orgID string, dailyUpdateID int64, topics []TopicInsert) error {
	if len(topics) == 0 {
		return nil
	}
	const query = `INSERT INTO daily_update_topics (org_id, daily_update_id, extracted_topic, topic_embedding) VALUES ($1, $2, $3, $4::vector)`
	batch := &pgx.Batch{}
	for _, t := range topics {
		batch.Queue(query, orgID, dailyUpdateID, t.ExtractedTopic, floatSliceToVectorLiteral(t.Embedding))
	}
	return q.SendBatch(ctx, batch).Close()
}

// FindLatestPerUserWithTopics returns today's most recent active update per
// visible user, with each update's extracted topics aggregated into a slice.
// A non-nil teamID restricts the result to members of that team; the RLS
// policy supplies the organization scoping.
func (r *DailyUpdateRepo) FindLatestPerUserWithTopics(ctx context.Context, q Querier, teamID *string) ([]DashboardRowWithTopics, error) {
	const query = `
		SELECT
			u.id, u.name, u.email,
			latest.update_text, latest.created_at,
			COALESCE(
				ARRAY_AGG(t.extracted_topic ORDER BY t.id) FILTER (WHERE t.id IS NOT NULL),
				'{}'
			) AS topics
		FROM users u
		LEFT JOIN LATERAL (
			SELECT id, update_text, created_at
			FROM daily_updates
			WHERE user_id = u.id
			  AND deleted_at IS NULL
			  AND created_at::date = NOW()::date
			ORDER BY created_at DESC
			LIMIT 1
		) latest ON TRUE
		LEFT JOIN daily_update_topics t ON t.daily_update_id = latest.id
		WHERE ($1::uuid IS NULL OR u.team_id = $1::uuid)
		GROUP BY u.id, u.name, u.email, latest.update_text, latest.created_at
		ORDER BY u.id`

	rows, err := q.Query(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DashboardRowWithTopics
	for rows.Next() {
		var row DashboardRowWithTopics
		if err := rows.Scan(
			&row.UserID, &row.Name, &row.Email,
			&row.UpdateText, &row.UpdateAt,
			&row.Topics,
		); err != nil {
			return nil, err
		}
		if row.Topics == nil {
			row.Topics = []string{}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// FindTodayTopicSimilarities returns upper-triangle pairwise cosine similarities
// between all distinct topic embeddings from today's updates. A non-nil teamID
// restricts the topics to updates authored by members of that team.
func (r *DailyUpdateRepo) FindTodayTopicSimilarities(ctx context.Context, q Querier, teamID *string) ([]TopicSimilarityRow, error) {
	const query = `
		WITH todays AS (
			SELECT DISTINCT ON (t.extracted_topic)
				t.id, t.extracted_topic, t.topic_embedding
			FROM daily_update_topics t
			JOIN daily_updates du ON du.id = t.daily_update_id
			JOIN users u ON u.id = du.user_id
			WHERE du.created_at::date = NOW()::date
			  AND du.deleted_at IS NULL
			  AND t.topic_embedding IS NOT NULL
			  AND ($1::uuid IS NULL OR u.team_id = $1::uuid)
			ORDER BY t.extracted_topic, t.id
		)
		SELECT
			a.extracted_topic,
			b.extracted_topic,
			1.0 - (a.topic_embedding <=> b.topic_embedding) AS similarity
		FROM todays a
		JOIN todays b ON a.extracted_topic < b.extracted_topic
		ORDER BY a.extracted_topic, b.extracted_topic`

	rows, err := q.Query(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TopicSimilarityRow
	for rows.Next() {
		var row TopicSimilarityRow
		if err := rows.Scan(&row.TopicA, &row.TopicB, &row.Similarity); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// floatSliceToVectorLiteral converts a float32 slice to a pgvector literal string
// of the form "[0.1,0.2,...]" suitable for casting with ::vector in SQL.
func floatSliceToVectorLiteral(v []float32) string {
	sb := strings.Builder{}
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%g", f))
	}
	sb.WriteByte(']')
	return sb.String()
}
