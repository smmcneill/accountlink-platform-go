package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
)

type (
	fakeClock struct{ now time.Time }

	fakeTx struct{}

	fakeTxManager struct{}

	fakeRepo struct {
		mu    sync.Mutex
		links map[uuid.UUID]domain.AccountLink
	}

	fakeIdem struct {
		mu      sync.Mutex
		records map[string]domain.IdempotencyRecord
	}

	fakeOutbox struct {
		mu     sync.Mutex
		events []domain.OutboxEvent
	}
)

func (f fakeClock) Now() time.Time { return f.now }

func (fakeTx) Commit(context.Context) error   { return nil }
func (fakeTx) Rollback(context.Context) error { return nil }

func (fakeTxManager) Begin(context.Context) (domain.Tx, error) { return fakeTx{}, nil }

func newFakeRepo() *fakeRepo { return &fakeRepo{links: map[uuid.UUID]domain.AccountLink{}} }

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.AccountLink, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	link, ok := r.links[id]

	return link, ok, nil
}

func (r *fakeRepo) Save(_ context.Context, _ domain.Tx, link domain.AccountLink) (domain.AccountLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.links[link.ID] = link

	return link, nil
}

func newFakeIdem() *fakeIdem { return &fakeIdem{records: map[string]domain.IdempotencyRecord{}} }

func (f *fakeIdem) FindByKey(_ context.Context, key string) (domain.IdempotencyRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	r, ok := f.records[key]

	return r, ok, nil
}

func (f *fakeIdem) TryInsert(_ context.Context, _ domain.Tx, rec domain.IdempotencyRecord) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.records[rec.Key]; exists {
		return false, nil
	}

	f.records[rec.Key] = rec

	return true, nil
}

func (f *fakeOutbox) Add(_ context.Context, _ domain.Tx, event domain.OutboxEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.events = append(f.events, event)

	return nil
}

func (f *fakeOutbox) FindUnpublishedForUpdateSkipLocked(_ context.Context, _ domain.Tx, _ int) ([]domain.OutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pending := make([]domain.OutboxEvent, 0)

	for _, evt := range f.events {
		if evt.PublishedAt == nil {
			pending = append(pending, evt)
		}
	}

	return pending, nil
}

func (f *fakeOutbox) MarkPublished(_ context.Context, _ domain.Tx, id uuid.UUID, publishedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.events {
		if f.events[i].ID == id {
			f.events[i].PublishedAt = &publishedAt
		}
	}

	return nil
}

func TestCreateWithSameKeySamePayloadReturnsSameResource(t *testing.T) {
	s := NewAccountLinkService(fakeTxManager{}, newFakeRepo(), newFakeIdem(), new(fakeOutbox), fakeClock{now: time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)})

	r1, err := s.Create(context.Background(), "idem-1", "user-123", "Chase")
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	r2, err := s.Create(context.Background(), "idem-1", "user-123", "Chase")
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}

	if !r1.Created {
		t.Fatalf("expected first create to be created")
	}

	if r2.Created {
		t.Fatalf("expected second create to be replay")
	}

	if r1.Link.ID != r2.Link.ID {
		t.Fatalf("expected same resource id")
	}
}

func TestCreateWithSameKeyDifferentPayloadReturnsConflict(t *testing.T) {
	s := NewAccountLinkService(fakeTxManager{}, newFakeRepo(), newFakeIdem(), new(fakeOutbox), fakeClock{now: time.Now().UTC()})

	_, err := s.Create(context.Background(), "idem-2", "user-123", "Chase")
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	_, err = s.Create(context.Background(), "idem-2", "user-999", "Chase")
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}
