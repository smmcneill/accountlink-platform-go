package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"accountlink-platform-go/internal/api"
	"accountlink-platform-go/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	router     *chi.Mux
	httpServer *http.Server
	logger     *slog.Logger
}

func New(addr string, logger *slog.Logger, handler *api.Handler) *Server {
	router := chi.NewRouter()
	router.Use(chimw.Recoverer)
	router.Use(middleware.Logging(logger))
	router.Get("/_health", handler.Health)
	router.Get("/account-links/{id}", handler.GetAccountLink)
	router.Post("/account-links", handler.CreateAccountLink)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		router:     router,
		httpServer: httpServer,
		logger:     logger,
	}
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
		return err
	}
}
