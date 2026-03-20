package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/domain"

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
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/account-links" && req.Method == http.MethodPost:
			h.CreateAccountLink(w, req)
		case req.URL.Path != "/account-links" && strings.HasPrefix(req.URL.Path, "/account-links/") && req.Method == http.MethodGet:
			h.GetAccountLink(w, req)
		case req.URL.Path == "/_health" && req.Method == http.MethodGet:
			h.Health(w, req)
		default:
			http.NotFound(w, req)
		}
	})
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

	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected health response content-type to be exactly text/plain, got %q", got)
	}
}

func TestGetAccountLinkReturns400OnInvalidID(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/account-links/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected application/json content-type, got %q", got)
	}
}

func TestGetAccountLinkReturns404WhenMissing(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/account-links/"+id.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetAccountLinkReturns500OnRepoError(t *testing.T) {
	svc := app.NewAccountLinkService(
		apiFakeTxManager{},
		&apiFakeGetByIDErrRepo{},
		newAPIFakeIdem(),
		new(apiFakeOutbox),
	)
	h := NewHandler(svc)
	router := testRouter(h)
	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/account-links/"+id.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetAccountLinkReturnsLinkOnSuccess(t *testing.T) {
	repo := newAPIFakeRepo()
	svc := app.NewAccountLinkService(
		apiFakeTxManager{},
		repo,
		newAPIFakeIdem(),
		new(apiFakeOutbox),
	)
	h := NewHandler(svc)
	router := testRouter(h)

	link, err := domain.NewAccountLink(uuid.New(), "user-123", "Chase", domain.LinkStatusActive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.links[link.ID] = link

	req := httptest.NewRequest(http.MethodGet, "/account-links/"+link.ID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected application/json content-type, got %q", got)
	}

	var gotLink domain.AccountLink
	if err := json.NewDecoder(w.Body).Decode(&gotLink); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if gotLink.ID != link.ID {
		t.Fatalf("expected id %s, got %s", link.ID, gotLink.ID)
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

	var result domain.AccountLink
	if err := json.NewDecoder(w1.Body).Decode(&result); err != nil {
		t.Fatalf("decode created link response: %v", err)
	}

	if result.ID == uuid.Nil {
		t.Fatalf("expected created link id")
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

func TestCreateRejectsWhitespaceUserID(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	raw := []byte(`{"userId":"  ", "externalInstitution":"Chase"}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRejectsWhitespaceExternalInstitution(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	raw := []byte(`{"userId":"user-123", "externalInstitution":"  "}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateBlankExternalInstitutionReturns400(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	raw := []byte(`{"userId":"user-123", "externalInstitution":""}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateReturns400OnMalformedJSON(t *testing.T) {
	h := NewHandler(testService())
	router := testRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/account-links", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var pd struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(w.Body).Decode(&pd); err != nil {
		t.Fatalf("decode problem details: %v", err)
	}

	if pd.Detail != "Malformed JSON" {
		t.Fatalf("expected malformed json detail, got %q", pd.Detail)
	}
}

func TestCreateReturns409OnIdempotencyConflict(t *testing.T) {
	svc := app.NewAccountLinkService(
		apiFakeTxManager{},
		newAPIFakeRepo(),
		&apiFakeConflictIdem{},
		new(apiFakeOutbox),
	)
	h := NewHandler(svc)
	router := testRouter(h)

	body := []byte(`{"userId":"user-123","externalInstitution":"Chase"}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "conflict-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateReturns500OnOutboxError(t *testing.T) {
	svc := app.NewAccountLinkService(
		apiFakeTxManager{},
		newAPIFakeRepo(),
		newAPIFakeIdem(),
		&apiFakeOutboxAddErr{},
	)
	h := NewHandler(svc)
	router := testRouter(h)
	body := []byte(`{"userId":"user-123","externalInstitution":"Chase"}`)
	req := httptest.NewRequest(http.MethodPost, "/account-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
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

func (apiFakeTxManager) Begin(context.Context) (app.Tx, error) { return apiFakeTx{}, nil }

func newAPIFakeRepo() *apiFakeRepo { return &apiFakeRepo{links: map[uuid.UUID]domain.AccountLink{}} }

func (r *apiFakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.AccountLink, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.links[id]

	return v, ok, nil
}

func (r *apiFakeRepo) Save(_ context.Context, _ app.Tx, link domain.AccountLink) (domain.AccountLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.links[link.ID] = link

	return link, nil
}

type apiFakeGetByIDErrRepo struct{}

func (r *apiFakeGetByIDErrRepo) FindByID(_ context.Context, _ uuid.UUID) (domain.AccountLink, bool, error) {
	return domain.AccountLink{}, false, errors.New("db failure")
}

func (r *apiFakeGetByIDErrRepo) Save(_ context.Context, _ app.Tx, _ domain.AccountLink) (domain.AccountLink, error) {
	return domain.AccountLink{}, nil
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

func (i *apiFakeIdem) TryInsert(_ context.Context, _ app.Tx, rec domain.IdempotencyRecord) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.records[rec.Key]; ok {
		return false, nil
	}

	i.records[rec.Key] = rec

	return true, nil
}

type apiFakeConflictIdem struct{}

func (apiFakeConflictIdem) FindByKey(_ context.Context, _ string) (domain.IdempotencyRecord, bool, error) {
	return domain.IdempotencyRecord{
		Key:           "conflict-key",
		RequestHash:   "different-hash",
		AccountLinkID: uuid.New(),
	}, true, nil
}

func (apiFakeConflictIdem) TryInsert(_ context.Context, _ app.Tx, _ domain.IdempotencyRecord) (bool, error) {
	return false, nil
}

func (*apiFakeOutbox) Add(context.Context, app.Tx, domain.OutboxEvent) error { return nil }

func (*apiFakeOutbox) FindUnpublishedForUpdateSkipLocked(context.Context, app.Tx, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (*apiFakeOutbox) MarkPublished(context.Context, app.Tx, uuid.UUID, time.Time) error {
	return nil
}

type apiFakeOutboxAddErr struct{}

func (apiFakeOutboxAddErr) Add(context.Context, app.Tx, domain.OutboxEvent) error {
	return errors.New("outbox failure")
}

func (apiFakeOutboxAddErr) FindUnpublishedForUpdateSkipLocked(context.Context, app.Tx, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (apiFakeOutboxAddErr) MarkPublished(context.Context, app.Tx, uuid.UUID, time.Time) error {
	return nil
}
