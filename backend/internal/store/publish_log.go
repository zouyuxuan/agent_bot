package store

import (
	"context"
	"time"

	"ai-bot-chain/backend/internal/domain"
)

// PublishLog persists publish results (e.g. training data tx hash) to an external DB.
// Implementations should be concurrency-safe.
type PublishLog interface {
	SaveTrainingPublish(ctx context.Context, r domain.PublishResult) error
	Close() error
}

type noopPublishLog struct{}

func NewNoopPublishLog() PublishLog { return &noopPublishLog{} }

func (n *noopPublishLog) SaveTrainingPublish(_ context.Context, _ domain.PublishResult) error {
	return nil
}
func (n *noopPublishLog) Close() error { return nil }

// Helper for callers: ensure PublishedAt is set.
func normalizePublishedAt(r domain.PublishResult) domain.PublishResult {
	if r.PublishedAt.IsZero() {
		r.PublishedAt = time.Now()
	}
	return r
}
