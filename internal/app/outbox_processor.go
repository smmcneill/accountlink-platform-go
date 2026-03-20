package app

import (
	"context"
	"io"
	"log/slog"
	"time"

	"accountlink-platform-go/internal/domain"
)

type (
	OutboxProcessor struct {
		txManager TxManager
		outbox    OutboxRepository
		publisher EventPublisher
		now       func() time.Time
		batchSize int
		pollDelay time.Duration
		logger    *slog.Logger
	}

	OutboxProcessorOption func(*OutboxProcessor)
)

func WithOutboxProcessorBatchSize(batchSize int) OutboxProcessorOption {
	return func(p *OutboxProcessor) {
		p.batchSize = batchSize
	}
}

func WithOutboxProcessorPollDelay(delay time.Duration) OutboxProcessorOption {
	return func(p *OutboxProcessor) {
		p.pollDelay = delay
	}
}

func WithOutboxProcessorLogger(logger *slog.Logger) OutboxProcessorOption {
	return func(p *OutboxProcessor) {
		if logger != nil {
			p.logger = logger
		}
	}
}

func WithOutboxProcessorNow(now func() time.Time) OutboxProcessorOption {
	return func(p *OutboxProcessor) {
		p.now = now
	}
}

func NewOutboxProcessor(
	txManager TxManager,
	outbox OutboxRepository,
	publisher EventPublisher,
	opts ...OutboxProcessorOption,
) *OutboxProcessor {
	p := &OutboxProcessor{
		txManager: txManager,
		outbox:    outbox,
		publisher: publisher,
		now:       func() time.Time { return time.Now().UTC() },
		batchSize: 10,
		pollDelay: time.Second,
		logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}

	for _, option := range opts {
		option(p)
	}

	return p
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
		now := p.now()
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
