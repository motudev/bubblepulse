// Package worker contains River background job workers.
package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/jobs"
)

// embeddingComputer generates a 384-dimensional embedding vector for the given text.
type embeddingComputer interface {
	Compute(text string) ([]float32, error)
}

// nlpUpdater is the data-access interface the NLP worker needs.
type nlpUpdater interface {
	FindUpdateTextByID(ctx context.Context, id int64) (string, error)
	SetUpdateEmbedding(ctx context.Context, id int64, embedding []float32) error
	InsertTopics(ctx context.Context, dailyUpdateID int64, topics []repository.TopicInsert) error
}

// NLPWorker processes a daily_updates row: generates embeddings and extracts topics.
type NLPWorker struct {
	river.WorkerDefaults[jobs.NLPProcessingArgs]
	repo     nlpUpdater
	embedder embeddingComputer
	parser   topicParser
}

// NewNLPWorker constructs an NLPWorker with its dependencies.
func NewNLPWorker(pool *pgxpool.Pool, embedder embeddingComputer, parser topicParser) *NLPWorker {
	return &NLPWorker{
		repo:     repository.NewDailyUpdateRepo(pool),
		embedder: embedder,
		parser:   parser,
	}
}

// Work is the River job handler. Returning a non-nil error schedules a retry.
func (w *NLPWorker) Work(ctx context.Context, job *river.Job[jobs.NLPProcessingArgs]) error {
	id := job.Args.DailyUpdateID

	text, err := w.repo.FindUpdateTextByID(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch update text %d: %w", id, err)
	}

	fullEmb, err := w.embedder.Compute(text)
	if err != nil {
		return fmt.Errorf("compute full embedding: %w", err)
	}
	if err := w.repo.SetUpdateEmbedding(ctx, id, fullEmb); err != nil {
		return fmt.Errorf("store update embedding: %w", err)
	}

	topics, err := extractTopics(ctx, w.embedder, w.parser, text)
	if err != nil {
		slog.Warn("nlp: topic extraction failed, skipping topics", "update_id", id, "error", err)
		return nil
	}
	if len(topics) == 0 {
		return nil
	}

	return w.repo.InsertTopics(ctx, id, topics)
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
