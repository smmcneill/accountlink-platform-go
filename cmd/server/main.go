package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"accountlink-platform-go/internal/api"
	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/config"
	"accountlink-platform-go/internal/db"
	"accountlink-platform-go/internal/domain"
	"accountlink-platform-go/internal/events"
	"accountlink-platform-go/internal/middleware"
	"accountlink-platform-go/internal/persistence"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		logger.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		logger.Error("db migration failed", "err", err)
		os.Exit(1)
	}

	txManager := persistence.NewTxManager(pool)
	repo := persistence.NewAccountLinkStore(pool)
	idem := persistence.NewIdempotencyStore(pool)
	outbox := persistence.NewOutboxStore()
	clock := app.RealClock{}

	svc := app.NewAccountLinkService(txManager, repo, idem, outbox, clock)
	publisher, err := buildPublisher(ctx, cfg, logger)
	if err != nil {
		logger.Error("event publisher setup failed", "err", err)
		os.Exit(1)
	}

	processor := app.NewOutboxProcessor(txManager, outbox, publisher, clock, cfg.OutboxPollBatch, cfg.OutboxPollDelay, logger)
	go processor.Start(ctx)

	h := api.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logging(logger))
	r.Mount("/", h.Routes())

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server_starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func buildPublisher(ctx context.Context, cfg config.Config, logger *slog.Logger) (domain.EventPublisher, error) {
	if cfg.EventTarget != "sns" {
		return events.NewLoggingPublisher(logger), nil
	}
	if cfg.SNSTopicARN == "" {
		return nil, fmt.Errorf("ACCOUNTLINK_SNS_TOPIC_ARN is required when EVENT_TARGET=sns")
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.SNSRegion),
	}
	if cfg.SNSEndpoint != "" {
		resolver := awsv2.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (awsv2.Endpoint, error) {
			if service == sns.ServiceID {
				return awsv2.Endpoint{URL: cfg.SNSEndpoint, SigningRegion: cfg.SNSRegion}, nil
			}
			return awsv2.Endpoint{}, &awsv2.EndpointNotFoundError{}
		})
		loadOptions = append(loadOptions,
			awsconfig.WithEndpointResolverWithOptions(resolver),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}
	return events.NewSNSPublisher(sns.NewFromConfig(awsCfg), cfg.SNSTopicARN, logger), nil
}
