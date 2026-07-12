package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/motudev/bubblepulse/internal/jobs"
)

// ErrUserNotFound is returned by Handle when the platform user has no matching
// registration in user_identities.
var ErrUserNotFound = errors.New("platform user not registered")

// IncomingMessage is the normalised representation of a user update from any platform.
type IncomingMessage struct {
	Provider       string // matches user_identities.provider, e.g. "slack", "teams"
	PlatformUserID string // matches user_identities.provider_id
	Text           string
}

// dailyUpdateInserter is the transactional write side of the daily update repository.
type dailyUpdateInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, userID int64, text string) (int64, error)
}

// jobEnqueuer is the River client subset required by MessageService.
type jobEnqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// MessageService handles the platform-agnostic core: user lookup, update insertion,
// and NLP job enqueueing — all within a single database transaction.
type MessageService struct {
	pool    *pgxpool.Pool
	updates dailyUpdateInserter
	queue   jobEnqueuer
}

// NewMessageService constructs a MessageService with its dependencies.
func NewMessageService(pool *pgxpool.Pool, updates dailyUpdateInserter, queue jobEnqueuer) *MessageService {
	return &MessageService{pool: pool, updates: updates, queue: queue}
}

// Handle resolves the platform user to an internal user ID, then within a single
// transaction inserts the daily update and enqueues the NLP processing job.
// Returns ErrUserNotFound if the platform user has no app registration.
func (s *MessageService) Handle(ctx context.Context, msg IncomingMessage) error {
	userID, err := s.lookupUser(ctx, msg.Provider, msg.PlatformUserID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("messaging: user lookup DB error", "provider", msg.Provider, "provider_user", msg.PlatformUserID, "error", err)
		}
		return ErrUserNotFound
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	id, err := s.updates.InsertTx(ctx, tx, userID, msg.Text)
	if err != nil {
		return fmt.Errorf("insert daily update: %w", err)
	}

	if _, err := s.queue.InsertTx(ctx, tx, jobs.NLPProcessingArgs{DailyUpdateID: id}, nil); err != nil {
		return fmt.Errorf("enqueue NLP job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Debug("message handled", "provider", msg.Provider, "user_id", userID, "update_id", id)
	return nil
}

// lookupUser returns the internal user ID for the given (provider, providerID) pair.
func (s *MessageService) lookupUser(ctx context.Context, provider, providerID string) (int64, error) {
	const q = `SELECT user_id FROM user_identities WHERE provider = $1 AND provider_id = $2`
	var id int64
	err := s.pool.QueryRow(ctx, q, provider, providerID).Scan(&id)
	return id, err
}
