package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AccountLinkRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (AccountLink, bool, error)
	Save(ctx context.Context, tx Tx, link AccountLink) (AccountLink, error)
}

type IdempotencyRepository interface {
	FindByKey(ctx context.Context, key string) (IdempotencyRecord, bool, error)
	TryInsert(ctx context.Context, tx Tx, rec IdempotencyRecord) (bool, error)
}

type OutboxRepository interface {
	Add(ctx context.Context, tx Tx, event OutboxEvent) error
	FindUnpublishedForUpdateSkipLocked(ctx context.Context, tx Tx, batchSize int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, tx Tx, id uuid.UUID, publishedAt time.Time) error
}

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TxManager interface {
	Begin(ctx context.Context) (Tx, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, event PublishedEvent) error
}
