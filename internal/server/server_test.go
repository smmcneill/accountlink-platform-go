package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"accountlink-platform-go/internal/api"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewRegistersRoutesAndMethodGuards(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(":", logger, api.NewHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/_health", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /_health expected status %d, got %d", http.StatusOK, w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/_health", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /_health expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
	if got := w.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}

	req = httptest.NewRequest(http.MethodGet, "/account-links", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /account-links expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recovered := recoverMiddleware(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	recovered.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if got := w.Body.String(); got != "internal server error" {
		t.Fatalf("expected body %q, got %q", "internal server error", got)
	}
}

func TestRunReturnsMyErrorWhenServerCannotStart(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New("localhost:bad", logger, api.NewHandler(nil))

	err := server.Run(context.Background())
	if err == nil {
		t.Fatal("expected listen failure")
	}

	var myErr MyError
	if !errors.As(err, &myErr) {
		t.Fatalf("expected MyError, got %T", err)
	}
}

func TestRunShutsDownOnContextCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(":0", logger, api.NewHandler(nil))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestRunCancelImmediate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New("", logger, api.NewHandler(nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
