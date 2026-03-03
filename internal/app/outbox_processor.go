package app

import (
	"context"
	"log/slog"
	"time"

	"accountlink-platform-go/internal/domain"
)

type OutboxProcessor struct {
	txManager domain.TxManager
	outbox    domain.OutboxRepository
	publisher domain.EventPublisher
	clock     Clock
	batchSize int
	pollDelay time.Duration
	logger    *slog.Logger
}

func NewOutboxProcessor(
	txManager domain.TxManager,
	outbox domain.OutboxRepository,
	publisher domain.EventPublisher,
	clock Clock,
	batchSize int,
	pollDelay time.Duration,
	logger *slog.Logger,
) *OutboxProcessor {
	return &OutboxProcessor{
		txManager: txManager,
		outbox:    outbox,
		publisher: publisher,
		clock:     clock,
		batchSize: batchSize,
		pollDelay: pollDelay,
		logger:    logger,
	}
}

func (p *OutboxProcessor) Start(ctx context.Context) {
	t := time.NewTicker(p.pollDelay)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.PublishOnce(ctx, p.batchSize); err != nil {
				p.logger.Error("outbox tick failed", "err", err)
			}
		}
	}
}

func (p *OutboxProcessor) PublishOnce(ctx context.Context, batchSize int) error {
	tx, err := p.txManager.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	events, err := p.outbox.FindUnpublishedForUpdateSkipLocked(ctx, tx, batchSize)
	if err != nil {
		return err
	}

	for _, event := range events {
		now := p.clock.Now()
		if err := p.publisher.Publish(ctx, domain.PublishedEvent{
			OutboxID:      event.ID,
			EventType:     event.EventType,
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			CreatedAt:     event.CreatedAt,
			PublishedAt:   now,
			Payload:       event.Payload,
		}); err != nil {
			return err
		}

		if err := p.outbox.MarkPublished(ctx, tx, event.ID, now); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
