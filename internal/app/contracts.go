package app

import (
	"context"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
)

type (
	Tx interface {
		Commit(ctx context.Context) error
		Rollback(ctx context.Context) error
	}

	TxManager interface {
		Begin(ctx context.Context) (Tx, error)
	}

	EventPublisher interface {
		Publish(ctx context.Context, event domain.PublishedEvent) error
	}

	AccountLinkRepository interface {
		FindByID(ctx context.Context, id uuid.UUID) (domain.AccountLink, bool, error)
		Save(ctx context.Context, tx Tx, link domain.AccountLink) (domain.AccountLink, error)
	}

	IdempotencyRepository interface {
		FindByKey(ctx context.Context, key string) (domain.IdempotencyRecord, bool, error)
		TryInsert(ctx context.Context, tx Tx, rec domain.IdempotencyRecord) (bool, error)
	}

	OutboxRepository interface {
		Add(ctx context.Context, tx Tx, event domain.OutboxEvent) error
		FindUnpublishedForUpdateSkipLocked(ctx context.Context, tx Tx, batchSize int) ([]domain.OutboxEvent, error)
		MarkPublished(ctx context.Context, tx Tx, id uuid.UUID, publishedAt time.Time) error
	}
)
