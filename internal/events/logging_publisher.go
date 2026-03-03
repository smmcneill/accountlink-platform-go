package events

import (
	"context"
	"log/slog"

	"accountlink-platform-go/internal/domain"
)

type LoggingPublisher struct {
	logger *slog.Logger
}

func NewLoggingPublisher(logger *slog.Logger) *LoggingPublisher {
	return &LoggingPublisher{logger: logger}
}

func (p *LoggingPublisher) Publish(_ context.Context, event domain.PublishedEvent) error {
	p.logger.Info("outbox_event_published",
		slog.String("event.type", event.EventType),
		slog.String("event.outbox_id", event.OutboxID.String()),
		slog.String("aggregate.type", event.AggregateType),
		slog.String("aggregate.id", event.AggregateID),
		slog.String("outbox.created_at", event.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")),
		slog.Int64("outbox.published_at", event.PublishedAt.UnixMilli()),
		slog.Int("payload.size", len(event.Payload)),
	)

	return nil
}
