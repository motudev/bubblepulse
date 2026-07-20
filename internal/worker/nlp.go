// Package worker contains River background job workers.
package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/jobs"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// embeddingComputer generates a 384-dimensional embedding vector for the given text.
type embeddingComputer interface {
	Compute(text string) ([]float32, error)
}

// nlpUpdater is the data-access interface the NLP worker needs. All methods
// touch RLS tables and therefore take a tenant-scoped Querier.
type nlpUpdater interface {
	FindUpdateTextByID(ctx context.Context, q repository.Querier, id int64) (string, error)
	SetUpdateEmbedding(ctx context.Context, q repository.Querier, id int64, embedding []float32) error
	InsertTopics(ctx context.Context, q repository.Querier, orgID string, dailyUpdateID int64, topics []repository.TopicInsert) error
}

// tenantTxRunner opens tenant-scoped transactions (satisfied by *tenancy.Runner).
type tenantTxRunner interface {
	RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// NLPWorker processes a daily_updates row: generates embeddings and extracts topics.
type NLPWorker struct {
	river.WorkerDefaults[jobs.NLPProcessingArgs]
	runner   tenantTxRunner
	repo     nlpUpdater
	embedder embeddingComputer
	parser   topicParser
}

// NewNLPWorker constructs an NLPWorker with its dependencies.
func NewNLPWorker(runner tenantTxRunner, repo nlpUpdater, embedder embeddingComputer, parser topicParser) *NLPWorker {
	return &NLPWorker{
		runner:   runner,
		repo:     repo,
		embedder: embedder,
		parser:   parser,
	}
}

// Work is the River job handler. Returning a non-nil error schedules a retry.
// All database access runs in tenant-scoped transactions bound to the job's
// OrgID, so the worker obeys the same RLS boundaries as the API layer.
func (w *NLPWorker) Work(ctx context.Context, job *river.Job[jobs.NLPProcessingArgs]) error {
	id := job.Args.DailyUpdateID
	orgID := job.Args.OrgID

	if orgID == "" {
		// Legacy job enqueued before multi-tenancy: no tenant to scope to.
		// Cancel instead of erroring so River doesn't retry it forever.
		slog.Warn("nlp: job without org_id, cancelling", "update_id", id)
		return river.JobCancel(fmt.Errorf("nlp job %d has no org_id", id))
	}
	ctx = tenancy.WithTenantID(ctx, orgID)

	// Short read transaction: the embedding computation below calls the ONNX
	// model and the NLP sidecar, and must not hold a database transaction open.
	var text string
	err := w.runner.RunTx(ctx, func(tx pgx.Tx) error {
		var txErr error
		text, txErr = w.repo.FindUpdateTextByID(ctx, tx, id)
		return txErr
	})
	if err != nil {
		return fmt.Errorf("fetch update text %d: %w", id, err)
	}

	fullEmb, err := w.embedder.Compute(text)
	if err != nil {
		return fmt.Errorf("compute full embedding: %w", err)
	}

	topics, err := extractTopics(ctx, w.embedder, w.parser, text)
	if err != nil {
		slog.Warn("nlp: topic extraction failed, skipping topics", "update_id", id, "error", err)
		topics = nil
	}

	return w.runner.RunTx(ctx, func(tx pgx.Tx) error {
		if err := w.repo.SetUpdateEmbedding(ctx, tx, id, fullEmb); err != nil {
			return fmt.Errorf("store update embedding: %w", err)
		}
		if len(topics) == 0 {
			return nil
		}
		return w.repo.InsertTopics(ctx, tx, orgID, id, topics)
	})
}

// extractTopics calls the NLP sidecar to get noun phrases, then embeds each one.
func extractTopics(ctx context.Context, embedder embeddingComputer, parser topicParser, text string) ([]repository.TopicInsert, error) {
	phrases, err := parser.ParseTopics(ctx, text)
	if err != nil {
		return nil, err
	}

	results := make([]repository.TopicInsert, 0, len(phrases))
	for _, phrase := range phrases {
		emb, err := embedder.Compute(phrase)
		if err != nil {
			slog.Warn("nlp: failed to compute embedding", "phrase", phrase, "error", err)
			continue
		}
		results = append(results, repository.TopicInsert{
			ExtractedTopic: phrase,
			Embedding:      emb,
		})
	}
	return results, nil
}
