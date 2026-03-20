package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")
	ErrNotFound            = errors.New("account link not found")
)

type (
	AccountLinkService struct {
		txManager TxManager
		repo      AccountLinkRepository
		idem      IdempotencyRepository
		outbox    OutboxRepository
		now       func() time.Time
		marshal   func(interface{}) ([]byte, error)
	}

	CreateAccountLinkResult struct {
		Link    domain.AccountLink
		Created bool
	}

	accountLinkCreatedPayload struct {
		ID                  uuid.UUID `json:"id"`
		UserID              string    `json:"userId"`
		ExternalInstitution string    `json:"externalInstitution"`
		Status              string    `json:"status"`
	}

	AccountLinkServiceOption func(*AccountLinkService)
)

func UTCNow() time.Time { return time.Now().UTC() }

func NewAccountLinkService(
	txManager TxManager,
	repo AccountLinkRepository,
	idem IdempotencyRepository,
	outbox OutboxRepository,
	opts ...AccountLinkServiceOption,
) *AccountLinkService {
	svc := &AccountLinkService{
		txManager: txManager,
		repo:      repo,
		idem:      idem,
		outbox:    outbox,
		now:       UTCNow,
		marshal:   json.Marshal,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
}

func WithAccountLinkServiceNow(now func() time.Time) AccountLinkServiceOption {
	return func(svc *AccountLinkService) {
		if now != nil {
			svc.now = now
		}
	}
}

func WithAccountLinkServiceMarshal(marshal func(interface{}) ([]byte, error)) AccountLinkServiceOption {
	return func(svc *AccountLinkService) {
		if marshal != nil {
			svc.marshal = marshal
		}
	}
}

func (s *AccountLinkService) GetByID(ctx context.Context, id uuid.UUID) (domain.AccountLink, error) {
	link, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.AccountLink{}, err
	}

	if !ok {
		return domain.AccountLink{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	return link, nil
}

func (s *AccountLinkService) Create(ctx context.Context, idemKey, userID, externalInstitution string) (CreateAccountLinkResult, error) {
	if idemKey == "" {
		return s.createNonIdempotent(ctx, userID, externalInstitution)
	}

	requestHash := sha256Hex(userID + "|" + externalInstitution)

	rec, found, err := s.idem.FindByKey(ctx, idemKey)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	if found {
		return s.replay(ctx, rec, requestHash)
	}

	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	created, err := s.createNew(ctx, tx, userID, externalInstitution)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	inserted, err := s.idem.TryInsert(ctx, tx, domain.IdempotencyRecord{
		Key:           idemKey,
		RequestHash:   requestHash,
		AccountLinkID: created.ID,
	})
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	if inserted {
		if err := s.writeAccountLinkCreatedOutbox(ctx, tx, created); err != nil {
			return CreateAccountLinkResult{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return CreateAccountLinkResult{}, err
		}

		return CreateAccountLinkResult{Link: created, Created: true}, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateAccountLinkResult{}, err
	}

	nowRec, nowFound, err := s.idem.FindByKey(ctx, idemKey)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	if !nowFound {
		return CreateAccountLinkResult{}, fmt.Errorf("idempotency key existed but could not be read: %s", idemKey)
	}

	return s.replay(ctx, nowRec, requestHash)
}

func (s *AccountLinkService) createNonIdempotent(ctx context.Context, userID, externalInstitution string) (CreateAccountLinkResult, error) {
	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	created, err := s.createNew(ctx, tx, userID, externalInstitution)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	if err := s.writeAccountLinkCreatedOutbox(ctx, tx, created); err != nil {
		return CreateAccountLinkResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateAccountLinkResult{}, err
	}

	return CreateAccountLinkResult{Link: created, Created: true}, nil
}

func (s *AccountLinkService) replay(ctx context.Context, rec domain.IdempotencyRecord, requestHash string) (CreateAccountLinkResult, error) {
	if rec.RequestHash != requestHash {
		return CreateAccountLinkResult{}, ErrIdempotencyConflict
	}

	link, ok, err := s.repo.FindByID(ctx, rec.AccountLinkID)
	if err != nil {
		return CreateAccountLinkResult{}, err
	}

	if !ok {
		return CreateAccountLinkResult{}, fmt.Errorf("idempotency record referenced missing AccountLink: %s", rec.AccountLinkID)
	}

	return CreateAccountLinkResult{Link: link, Created: false}, nil
}

func (s *AccountLinkService) createNew(ctx context.Context, tx Tx, userID, externalInstitution string) (domain.AccountLink, error) {
	link, err := domain.NewAccountLink(uuid.New(), userID, externalInstitution, domain.LinkStatusPending)
	if err != nil {
		return domain.AccountLink{}, err
	}

	return s.repo.Save(ctx, tx, link)
}

func (s *AccountLinkService) writeAccountLinkCreatedOutbox(ctx context.Context, tx Tx, link domain.AccountLink) error {
	payload, err := s.marshal(accountLinkCreatedPayload{
		ID:                  link.ID,
		UserID:              link.UserID,
		ExternalInstitution: link.ExternalInstitution,
		Status:              string(link.Status),
	})
	if err != nil {
		return fmt.Errorf("failed to serialize outbox payload: %w", err)
	}

	return s.outbox.Add(ctx, tx, domain.OutboxEvent{
		ID:            uuid.New(),
		EventType:     "AccountLinkCreated",
		AggregateType: "AccountLink",
		AggregateID:   link.ID.String(),
		Payload:       string(payload),
		CreatedAt:     s.now(),
		PublishedAt:   nil,
	})
}

func sha256Hex(v string) string {
	s := sha256.Sum256([]byte(v))
	return hex.EncodeToString(s[:])
}
