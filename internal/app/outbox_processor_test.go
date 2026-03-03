package app

import (
	"context"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"
	domainmocks "accountlink-platform-go/internal/domain/mocks"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestPublishOnceMarksPublished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := domainmocks.NewMockTxManager(ctrl)
	tx := domainmocks.NewMockTx(ctrl)
	outbox := domainmocks.NewMockOutboxRepository(ctrl)
	publisher := domainmocks.NewMockEventPublisher(ctrl)

	outboxID := uuid.New()
	createdAt := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	publishedAt := createdAt.Add(5 * time.Second)
	event := domain.OutboxEvent{
		ID:            outboxID,
		EventType:     "AccountLinkCreated",
		AggregateType: "AccountLink",
		AggregateID:   "agg-1",
		Payload:       `{"x":1}`,
		CreatedAt:     createdAt,
	}

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 10).Return([]domain.OutboxEvent{event}, nil),
		publisher.EXPECT().Publish(gomock.Any(), domain.PublishedEvent{
			OutboxID:      outboxID,
			EventType:     event.EventType,
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			CreatedAt:     createdAt,
			PublishedAt:   publishedAt,
			Payload:       event.Payload,
		}).Return(nil),
		outbox.EXPECT().MarkPublished(gomock.Any(), tx, outboxID, publishedAt).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(txManager, outbox, publisher, 10, time.Second, nil)
	p.now = func() time.Time { return publishedAt }

	if err := p.PublishOnce(context.Background(), 10); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}
}
