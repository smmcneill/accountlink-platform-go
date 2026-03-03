package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"
	domainmocks "accountlink-platform-go/internal/domain/mocks"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestCreateWithSameKeySamePayloadReturnsSameResource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := domainmocks.NewMockTxManager(ctrl)
	tx := domainmocks.NewMockTx(ctrl)
	repo := domainmocks.NewMockAccountLinkRepository(ctrl)
	idem := domainmocks.NewMockIdempotencyRepository(ctrl)
	outbox := domainmocks.NewMockOutboxRepository(ctrl)

	fixedNow := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	createdID := uuid.New()
	created := domain.AccountLink{
		ID:                  createdID,
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	requestHash := sha256Hex("user-123|Chase")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-1").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.AssignableToTypeOf(domain.IdempotencyRecord{})).
			DoAndReturn(func(_ context.Context, _ domain.Tx, rec domain.IdempotencyRecord) (bool, error) {
				if rec.Key != "idem-1" || rec.RequestHash != requestHash || rec.AccountLinkID != createdID {
					t.Fatalf("unexpected idempotency record: %+v", rec)
				}

				return true, nil
			}),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.AssignableToTypeOf(domain.OutboxEvent{})).
			DoAndReturn(func(_ context.Context, _ domain.Tx, event domain.OutboxEvent) error {
				if event.CreatedAt != fixedNow {
					t.Fatalf("unexpected event timestamp: got %v want %v", event.CreatedAt, fixedNow)
				}

				return nil
			}),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
		idem.EXPECT().FindByKey(gomock.Any(), "idem-1").Return(domain.IdempotencyRecord{
			Key:           "idem-1",
			RequestHash:   requestHash,
			AccountLinkID: createdID,
		}, true, nil),
		repo.EXPECT().FindByID(gomock.Any(), createdID).Return(created, true, nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	svc.now = func() time.Time { return fixedNow }

	r1, err := svc.Create(context.Background(), "idem-1", "user-123", "Chase")
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	r2, err := svc.Create(context.Background(), "idem-1", "user-123", "Chase")
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := domainmocks.NewMockTxManager(ctrl)
	tx := domainmocks.NewMockTx(ctrl)
	repo := domainmocks.NewMockAccountLinkRepository(ctrl)
	idem := domainmocks.NewMockIdempotencyRepository(ctrl)
	outbox := domainmocks.NewMockOutboxRepository(ctrl)

	createdID := uuid.New()
	created := domain.AccountLink{
		ID:                  createdID,
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	initialHash := sha256Hex("user-123|Chase")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-2").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).Return(true, nil),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
		idem.EXPECT().FindByKey(gomock.Any(), "idem-2").Return(domain.IdempotencyRecord{
			Key:           "idem-2",
			RequestHash:   initialHash,
			AccountLinkID: createdID,
		}, true, nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)

	_, err := svc.Create(context.Background(), "idem-2", "user-123", "Chase")
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	_, err = svc.Create(context.Background(), "idem-2", "user-999", "Chase")
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}
