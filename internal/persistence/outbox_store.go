package persistence

import (
	"context"
	"time"

	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
)

type OutboxStore struct{}

func NewOutboxStore() *OutboxStore {
	return new(OutboxStore)
}

func (s *OutboxStore) Add(ctx context.Context, tx app.Tx, event domain.OutboxEvent) error {
	const q = `
INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, created_at, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := unwrapTx(tx).Exec(
		ctx,
		q,
		event.ID,
		event.EventType,
		event.AggregateType,
		event.AggregateID,
		event.Payload,
		event.CreatedAt,
		event.PublishedAt,
	)

	return err
}

func (s *OutboxStore) FindUnpublishedForUpdateSkipLocked(ctx context.Context, tx app.Tx, batchSize int) ([]domain.OutboxEvent, error) {
	const q = `
SELECT id, event_type, aggregate_type, aggregate_id, payload, created_at, published_at
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED`

	rows, err := unwrapTx(tx).Query(ctx, q, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.OutboxEvent, 0)

	for rows.Next() {
		var evt domain.OutboxEvent
		if err := rows.Scan(
			&evt.ID,
			&evt.EventType,
			&evt.AggregateType,
			&evt.AggregateID,
			&evt.Payload,
			&evt.CreatedAt,
			&evt.PublishedAt,
		); err != nil {
			return nil, err
		}

		out = append(out, evt)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *OutboxStore) MarkPublished(ctx context.Context, tx app.Tx, id uuid.UUID, publishedAt time.Time) error {
	const q = `UPDATE outbox_events SET published_at = $2 WHERE id = $1`

	_, err := unwrapTx(tx).Exec(ctx, q, id, publishedAt)

	return err
}
