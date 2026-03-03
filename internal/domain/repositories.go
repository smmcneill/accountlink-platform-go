package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type (
	AccountLinkRepository interface {
		FindByID(ctx context.Context, id uuid.UUID) (AccountLink, bool, error)
		Save(ctx context.Context, tx Tx, link AccountLink) (AccountLink, error)
	}

	IdempotencyRepository interface {
		FindByKey(ctx context.Context, key string) (IdempotencyRecord, bool, error)
		TryInsert(ctx context.Context, tx Tx, rec IdempotencyRecord) (bool, error)
	}

	OutboxRepository interface {
		Add(ctx context.Context, tx Tx, event OutboxEvent) error
		FindUnpublishedForUpdateSkipLocked(ctx context.Context, tx Tx, batchSize int) ([]OutboxEvent, error)
		MarkPublished(ctx context.Context, tx Tx, id uuid.UUID, publishedAt time.Time) error
	}

	Tx interface {
		Commit(ctx context.Context) error
		Rollback(ctx context.Context) error
	}

	TxManager interface {
		Begin(ctx context.Context) (Tx, error)
	}

	EventPublisher interface {
		Publish(ctx context.Context, event PublishedEvent) error
	}
)
