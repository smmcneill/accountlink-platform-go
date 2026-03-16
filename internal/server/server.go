package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"accountlink-platform-go/internal/api"
	"accountlink-platform-go/internal/middleware"
)

type (
	Server struct {
		router     *http.ServeMux
		httpServer *http.Server
		logger     *slog.Logger
	}

	MyError struct{ error }
)

func (e MyError) Error() string {
	return e.error.Error()
}

func New(addr string, logger *slog.Logger, handler *api.Handler) *Server {
	mux := http.NewServeMux()
	mux.Handle("/_health", enforceMethod(http.MethodGet, handler.Health, logger))
	mux.Handle("/account-links/", enforceMethod(http.MethodGet, handler.GetAccountLink, logger))
	mux.Handle("/account-links", enforceMethod(http.MethodPost, handler.CreateAccountLink, logger))

	h := middleware.Logging(logger)(mux)
	h = recoverMiddleware(logger)(h)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		router:     mux,
		httpServer: httpServer,
		logger:     logger,
	}
}

func recoverMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("http panic",
						"err", recovered,
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)

					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func enforceMethod(method string, next http.HandlerFunc, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("method_invoked", "method", r.Method, "path", r.URL.Path, "expected", method)
		if r.Method != method {
			w.Header().Set("Allow", method)
			logger.Warn("method_not_allowed", "method", r.Method, "path", r.URL.Path, "expected", method)
			http.Error(w, fmt.Sprintf("%s not allowed for this route", r.Method), http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("server_starting", "addr", s.httpServer.Addr)

		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		// return err
		return MyError{err}
	}
}
