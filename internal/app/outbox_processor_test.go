package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestPublishOnceMarksPublished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

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

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(10),
		WithOutboxProcessorPollDelay(time.Second),
		WithOutboxProcessorNow(func() time.Time { return publishedAt }),
	)

	if err := p.PublishOnce(context.Background(), 10); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}
}

func TestNewOutboxProcessorDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 10).Return(nil, nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
	)
	if err := p.PublishOnce(context.Background(), 10); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}

}

func TestOutboxProcessorStartUsesDefaultBatchSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	batchSizeCh := make(chan int, 1)
	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil).AnyTimes(),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ Tx, batchSize int) ([]domain.OutboxEvent, error) {
				batchSizeCh <- batchSize
				return nil, nil
			}),
		tx.EXPECT().Commit(gomock.Any()).Return(nil).AnyTimes(),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes(),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorPollDelay(5*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Start(ctx)
		close(done)
	}()

	select {
	case size := <-batchSizeCh:
		if size != 10 {
			t.Fatalf("expected default batch size 10, got %d", size)
		}
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("timed out waiting for processor tick")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor to stop")
	}
}

func TestOutboxProcessorStartUsesConfiguredBatchSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	batchSizeCh := make(chan int, 1)
	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil).AnyTimes(),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ Tx, batchSize int) ([]domain.OutboxEvent, error) {
				batchSizeCh <- batchSize
				return nil, nil
			}),
		tx.EXPECT().Commit(gomock.Any()).Return(nil).AnyTimes(),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes(),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(7),
		WithOutboxProcessorPollDelay(5*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Start(ctx)
		close(done)
	}()

	select {
	case size := <-batchSizeCh:
		if size != 7 {
			t.Fatalf("expected configured batch size 7, got %d", size)
		}
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("timed out waiting for processor tick")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor to stop")
	}
}

func TestWithOutboxProcessorBatchSizeOption(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 7).Return(nil, nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(7),
	)
	if err := p.PublishOnce(context.Background(), 7); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}
}

func TestWithOutboxProcessorNowOption(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	outboxID := uuid.New()
	createdAt := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	publishedAt := createdAt.Add(3 * time.Minute)
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
		publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil),
		outbox.EXPECT().MarkPublished(gomock.Any(), tx, outboxID, publishedAt).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorNow(func() time.Time { return publishedAt }),
	)
	if err := p.PublishOnce(context.Background(), 10); err != nil {
		t.Fatalf("publish once failed: %v", err)
	}
}

func TestPublishOnceReturnsErrorWhenTransactionBeginFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	beginErr := errors.New("begin failed")
	txManager.EXPECT().Begin(gomock.Any()).Return(nil, beginErr)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(10),
		WithOutboxProcessorPollDelay(time.Second),
	)
	if err := p.PublishOnce(context.Background(), 10); !errors.Is(err, beginErr) {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestPublishOnceReturnsErrorWhenFindFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	findErr := errors.New("find failed")
	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 10).Return(nil, findErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(10),
		WithOutboxProcessorPollDelay(time.Second),
	)
	if err := p.PublishOnce(context.Background(), 10); !errors.Is(err, findErr) {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestPublishOnceReturnsErrorWhenPublishFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	event := domain.OutboxEvent{
		ID:            uuid.New(),
		EventType:     "AccountLinkCreated",
		AggregateType: "AccountLink",
		AggregateID:   "agg-1",
		Payload:       `{"x":1}`,
		CreatedAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	}
	publishErr := errors.New("publish failed")

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 10).Return([]domain.OutboxEvent{event}, nil),
		publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(publishErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(10),
		WithOutboxProcessorPollDelay(time.Second),
		WithOutboxProcessorNow(func() time.Time { return time.Now() }),
	)
	if err := p.PublishOnce(context.Background(), 10); !errors.Is(err, publishErr) {
		t.Fatalf("expected publish error, got %v", err)
	}
}

func TestPublishOnceReturnsErrorWhenMarkPublishedFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	event := domain.OutboxEvent{
		ID:            uuid.New(),
		EventType:     "AccountLinkCreated",
		AggregateType: "AccountLink",
		AggregateID:   "agg-1",
		Payload:       `{"x":1}`,
		CreatedAt:     time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	}
	markErr := errors.New("mark failed")

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		outbox.EXPECT().FindUnpublishedForUpdateSkipLocked(gomock.Any(), tx, 10).Return([]domain.OutboxEvent{event}, nil),
		publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil),
		outbox.EXPECT().MarkPublished(gomock.Any(), tx, event.ID, gomock.Any()).Return(markErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(10),
		WithOutboxProcessorPollDelay(time.Second),
	)
	if err := p.PublishOnce(context.Background(), 10); !errors.Is(err, markErr) {
		t.Fatalf("expected mark published error, got %v", err)
	}
}

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(name string) slog.Handler {
	return h
}

func TestStartLogsPublishErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	outbox := NewMockOutboxRepository(ctrl)
	publisher := NewMockEventPublisher(ctrl)

	beginErr := errors.New("begin failed")
	txManager.EXPECT().Begin(gomock.Any()).Return(nil, beginErr).AnyTimes()

	handler := &recordingHandler{}
	logger := slog.New(handler)
	p := NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		WithOutboxProcessorBatchSize(1),
		WithOutboxProcessorPollDelay(5*time.Millisecond),
		WithOutboxProcessorLogger(logger),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Start(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor stop")
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.records) == 0 {
		t.Fatal("expected at least one log record from PublishOnce error")
	}
}
