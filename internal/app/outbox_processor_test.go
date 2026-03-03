package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
)

type fakePublisher struct {
	published []domain.PublishedEvent
}

func (f *fakePublisher) Publish(_ context.Context, evt domain.PublishedEvent) error {
	f.published = append(f.published, evt)
	return nil
}

func TestPublishOnceMarksPublished(t *testing.T) {
	outbox := &fakeOutbox{events: []domain.OutboxEvent{{
		ID:            uuid.New(),
		EventType:     "AccountLinkCreated",
		AggregateType: "AccountLink",
		AggregateID:   "agg-1",
		Payload:       `{"x":1}`,
		CreatedAt:     time.Now().UTC(),
	}}}
	pub := &fakePublisher{}
	p := NewOutboxProcessor(fakeTxManager{}, outbox, pub, fakeClock{now: time.Now().UTC()}, 10, time.Second, slog.Default())

	if err := p.PublishOnce(context.Background(), 10); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}

	if len(pub.published) != 1 {
		t.Fatalf("expected one published event")
	}

	pending, _ := outbox.FindUnpublishedForUpdateSkipLocked(context.Background(), fakeTx{}, 10)
	if len(pending) != 0 {
		t.Fatalf("expected no pending events after publish")
	}
}
