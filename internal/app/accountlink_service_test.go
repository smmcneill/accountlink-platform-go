package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func testSHA256Hex(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func TestCreateWithSameKeySamePayloadReturnsSameResource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	fixedNow := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	createdID := uuid.New()
	created := domain.AccountLink{
		ID:                  createdID,
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	requestHash := testSHA256Hex("user-123|Chase")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-1").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.AssignableToTypeOf(domain.IdempotencyRecord{})).
			DoAndReturn(func(_ context.Context, _ Tx, rec domain.IdempotencyRecord) (bool, error) {
				if rec.Key != "idem-1" || rec.RequestHash != requestHash || rec.AccountLinkID != createdID {
					t.Fatalf("unexpected idempotency record: %+v", rec)
				}

				return true, nil
			}),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.AssignableToTypeOf(domain.OutboxEvent{})).
			DoAndReturn(func(_ context.Context, _ Tx, event domain.OutboxEvent) error {
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

	svc := NewAccountLinkService(
		txManager,
		repo,
		idem,
		outbox,
		WithAccountLinkServiceNow(func() time.Time { return fixedNow }),
	)

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

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	createdID := uuid.New()
	created := domain.AccountLink{
		ID:                  createdID,
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	initialHash := testSHA256Hex("user-123|Chase")

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

func TestGetByIDReturnsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	targetID := uuid.New()

	repo.EXPECT().FindByID(gomock.Any(), targetID).Return(domain.AccountLink{}, false, nil)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.GetByID(context.Background(), targetID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByIDPropagatesRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	targetID := uuid.New()
	expectedErr := fmt.Errorf("db down")

	repo.EXPECT().FindByID(gomock.Any(), targetID).Return(domain.AccountLink{}, false, expectedErr)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.GetByID(context.Background(), targetID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestCreateWithNoIdempotencyKeyUsesNonIdempotentPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	publishedAt := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(
		txManager,
		repo,
		idem,
		outbox,
		WithAccountLinkServiceNow(func() time.Time { return publishedAt }),
	)
	_, err := svc.Create(context.Background(), "", "user-123", "Chase")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestCreateReturnsErrorFromIdempotencyLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	lookupErr := fmt.Errorf("lookup failed")
	idem.EXPECT().FindByKey(gomock.Any(), "idem-3").Return(domain.IdempotencyRecord{}, false, lookupErr)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-3", "user-123", "Chase")
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected %v, got %v", lookupErr, err)
	}
}

func TestCreateBeginsTransactionAfterSuccessfulIdempotencyLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	idem.EXPECT().FindByKey(gomock.Any(), "idem-4").Return(domain.IdempotencyRecord{}, false, nil)
	beginErr := fmt.Errorf("begin failed")
	txManager.EXPECT().Begin(gomock.Any()).Return((*MockTx)(nil), beginErr)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-4", "user-123", "Chase")
	if !errors.Is(err, beginErr) {
		t.Fatalf("expected %v, got %v", beginErr, err)
	}
}

func TestCreateReturnsErrorWhenSavingAccountLinkFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-5").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(domain.AccountLink{}, fmt.Errorf("save failed")),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-5", "user-123", "Chase")
	if err == nil || err.Error() != "save failed" {
		t.Fatalf("expected save failed, got %v", err)
	}
}

func TestCreateReturnsErrorWhenIdempotencyInsertFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-6").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).Return(false, fmt.Errorf("insert failed")),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-6", "user-123", "Chase")
	if err == nil || err.Error() != "insert failed" {
		t.Fatalf("expected insert failed, got %v", err)
	}
}

func TestCreateReturnsErrorWhenInsertLostCommitFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}

	commitErr := fmt.Errorf("commit failed")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-11").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).Return(false, nil),
		tx.EXPECT().Commit(gomock.Any()).Return(commitErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-11", "user-123", "Chase")
	if err == nil || err.Error() != commitErr.Error() {
		t.Fatalf("expected commit failed, got %v", err)
	}
}

func TestCreateReturnsErrorWhenInsertLostReplayLookupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}

	lookupErr := fmt.Errorf("lookup failed")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-12").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).Return(false, nil),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		idem.EXPECT().FindByKey(gomock.Any(), "idem-12").Return(domain.IdempotencyRecord{}, false, lookupErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-12", "user-123", "Chase")
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected %v, got %v", lookupErr, err)
	}
}

func TestCreateReturnsErrorWhenOutboxWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	requestHash := testSHA256Hex("user-123|Chase")

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-7").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ Tx, rec domain.IdempotencyRecord) (bool, error) {
				if rec.Key != "idem-7" || rec.RequestHash != requestHash || rec.AccountLinkID != created.ID {
					t.Fatalf("unexpected record: %+v", rec)
				}
				return true, nil
			}),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(fmt.Errorf("outbox failed")),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-7", "user-123", "Chase")
	if err == nil || err.Error() != "outbox failed" {
		t.Fatalf("expected outbox failed, got %v", err)
	}
}

func TestCreateNonIdempotentReturnsErrorWhenTransactionBeginFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	beginErr := fmt.Errorf("begin failed")
	txManager.EXPECT().Begin(gomock.Any()).Return((*MockTx)(nil), beginErr)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "", "user-123", "Chase")
	if !errors.Is(err, beginErr) {
		t.Fatalf("expected %v, got %v", beginErr, err)
	}
}

func TestCreateNonIdempotentReturnsErrorWhenInputInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil)
	tx.EXPECT().Rollback(gomock.Any()).Return(nil)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "", "", "Chase")
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCreateNonIdempotentReturnsErrorWhenOutboxWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	outboxErr := fmt.Errorf("outbox add failed")

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(outboxErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "", "user-123", "Chase")
	if !errors.Is(err, outboxErr) {
		t.Fatalf("expected %v, got %v", outboxErr, err)
	}
}

func TestCreateNonIdempotentReturnsErrorWhenCommitFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	commitErr := fmt.Errorf("commit failed")

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(commitErr),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "", "user-123", "Chase")
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected %v, got %v", commitErr, err)
	}
}

func TestCreateNonIdempotentReturnsErrorWhenMarshalOutboxPayloadFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}
	marshalErr := fmt.Errorf("marshal failed")

	gomock.InOrder(
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(
		txManager,
		repo,
		idem,
		outbox,
		WithAccountLinkServiceMarshal(func(_ interface{}) ([]byte, error) { return nil, marshalErr }),
	)

	_, err := svc.Create(context.Background(), "", "user-123", "Chase")
	if !errors.Is(err, marshalErr) {
		t.Fatalf("expected %v, got %v", marshalErr, err)
	}
}

func TestCreateReturnsErrorWhenInsertedCommitFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	tx := NewMockTx(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	created := domain.AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              domain.LinkStatusPending,
	}

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-8").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(created, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).Return(true, nil),
		outbox.EXPECT().Add(gomock.Any(), tx, gomock.Any()).Return(nil),
		tx.EXPECT().Commit(gomock.Any()).Return(fmt.Errorf("commit failed")),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-8", "user-123", "Chase")
	if err == nil || err.Error() != "commit failed" {
		t.Fatalf("expected commit failed, got %v", err)
	}
}

func TestCreateReturnsErrorWhenReplayingRecordIsMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	createdID := uuid.New()
	requestHash := testSHA256Hex("user-123|Chase")
	rec := domain.IdempotencyRecord{
		Key:           "idem-9",
		RequestHash:   requestHash,
		AccountLinkID: createdID,
	}
	tx := NewMockTx(ctrl)

	gomock.InOrder(
		idem.EXPECT().FindByKey(gomock.Any(), "idem-9").Return(domain.IdempotencyRecord{}, false, nil),
		txManager.EXPECT().Begin(gomock.Any()).Return(tx, nil),
		// Create succeeds.
		repo.EXPECT().Save(gomock.Any(), tx, gomock.Any()).Return(domain.AccountLink{
			ID:                  createdID,
			UserID:              "user-123",
			ExternalInstitution: "Chase",
			Status:              domain.LinkStatusPending,
		}, nil),
		idem.EXPECT().TryInsert(gomock.Any(), tx, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ Tx, got domain.IdempotencyRecord) (bool, error) {
				if got.Key != rec.Key || got.RequestHash != rec.RequestHash || got.AccountLinkID != rec.AccountLinkID {
					t.Fatalf("unexpected record: %+v", got)
				}
				return false, nil
			}),
		tx.EXPECT().Commit(gomock.Any()).Return(nil),
		idem.EXPECT().FindByKey(gomock.Any(), "idem-9").Return(domain.IdempotencyRecord{
			Key:           rec.Key,
			RequestHash:   rec.RequestHash,
			AccountLinkID: createdID,
		}, false, nil),
		tx.EXPECT().Rollback(gomock.Any()).Return(nil),
	)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-9", "user-123", "Chase")
	if err == nil {
		t.Fatalf("expected missing replay record error")
	}
}

func TestReplayReturnsErrorWhenAccountLinkMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	rec := domain.IdempotencyRecord{
		Key:           "idem-10",
		RequestHash:   testSHA256Hex("user-123|Chase"),
		AccountLinkID: uuid.New(),
	}

	idem.EXPECT().FindByKey(gomock.Any(), "idem-10").Return(rec, true, nil)
	repo.EXPECT().FindByID(gomock.Any(), rec.AccountLinkID).Return(domain.AccountLink{}, false, nil)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-10", "user-123", "Chase")
	if err == nil {
		t.Fatalf("expected missing account link error")
	}
}

func TestReplayReturnsErrorWhenFindByIDFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := NewMockTxManager(ctrl)
	repo := NewMockAccountLinkRepository(ctrl)
	idem := NewMockIdempotencyRepository(ctrl)
	outbox := NewMockOutboxRepository(ctrl)

	rec := domain.IdempotencyRecord{
		Key:           "idem-13",
		RequestHash:   testSHA256Hex("user-123|Chase"),
		AccountLinkID: uuid.New(),
	}
	findErr := fmt.Errorf("repo read failed")

	idem.EXPECT().FindByKey(gomock.Any(), "idem-13").Return(rec, true, nil)
	repo.EXPECT().FindByID(gomock.Any(), rec.AccountLinkID).Return(domain.AccountLink{}, false, findErr)

	svc := NewAccountLinkService(txManager, repo, idem, outbox)
	_, err := svc.Create(context.Background(), "idem-13", "user-123", "Chase")
	if !errors.Is(err, findErr) {
		t.Fatalf("expected %v, got %v", findErr, err)
	}
}
