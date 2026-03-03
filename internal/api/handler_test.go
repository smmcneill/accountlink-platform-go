package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func testService() *app.AccountLinkService {
	return app.NewAccountLinkService(
		apiFakeTxManager{},
		newAPIFakeRepo(),
		newAPIFakeIdem(),
		new(apiFakeOutbox),
	)
}

func testRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/_health", h.Health)
	r.Get("/account-links/{id}", h.GetAccountLink)
	r.Post("/account-links", h.CreateAccountLink)

	return r
}

func TestHealthReturnsOK(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/_health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("expected 200/ok, got %d/%q", w.Code, w.Body.String())
	}
}

func TestCreateReplayReturns200(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)

	body := map[string]string{"userId": "user-123", "externalInstitution": "Chase"}
	raw, _ := json.Marshal(body)

	req1 := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "k-123")

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "k-123")

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 replay, got %d", w2.Code)
	}

	if w2.Header().Get("Location") == "" {
		t.Fatalf("expected location header")
	}
}

func TestCreateBlankUserIDReturns400(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	raw := []byte(`{"userId":"", "externalInstitution":"Chase"}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

type (
	apiFakeTx struct{}

	apiFakeTxManager struct{}

	apiFakeRepo struct {
		mu    sync.Mutex
		links map[uuid.UUID]domain.AccountLink
	}

	apiFakeIdem struct {
		mu      sync.Mutex
		records map[string]domain.IdempotencyRecord
	}

	apiFakeOutbox struct{}
)

func (apiFakeTx) Commit(context.Context) error   { return nil }
func (apiFakeTx) Rollback(context.Context) error { return nil }

func (apiFakeTxManager) Begin(context.Context) (domain.Tx, error) { return apiFakeTx{}, nil }

func newAPIFakeRepo() *apiFakeRepo { return &apiFakeRepo{links: map[uuid.UUID]domain.AccountLink{}} }

func (r *apiFakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.AccountLink, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.links[id]

	return v, ok, nil
}
func (r *apiFakeRepo) Save(_ context.Context, _ domain.Tx, link domain.AccountLink) (domain.AccountLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.links[link.ID] = link

	return link, nil
}

func newAPIFakeIdem() *apiFakeIdem {
	return &apiFakeIdem{records: map[string]domain.IdempotencyRecord{}}
}

func (i *apiFakeIdem) FindByKey(_ context.Context, key string) (domain.IdempotencyRecord, bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	r, ok := i.records[key]

	return r, ok, nil
}

func (i *apiFakeIdem) TryInsert(_ context.Context, _ domain.Tx, rec domain.IdempotencyRecord) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.records[rec.Key]; ok {
		return false, nil
	}

	i.records[rec.Key] = rec

	return true, nil
}

func (*apiFakeOutbox) Add(context.Context, domain.Tx, domain.OutboxEvent) error { return nil }
func (*apiFakeOutbox) FindUnpublishedForUpdateSkipLocked(context.Context, domain.Tx, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}
func (*apiFakeOutbox) MarkPublished(context.Context, domain.Tx, uuid.UUID, time.Time) error {
	return nil
}
