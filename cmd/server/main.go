package main

import (
	"context"
	"fmt"
	"log/slog"
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
	"accountlink-platform-go/internal/persistence"
	"accountlink-platform-go/internal/server"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("db connect failed: %w", err)
	}

	defer pool.Close()

	if err := migrateWithRetry(ctx, pool, cfg.DBStartupMaxWait, cfg.DBStartupRetry, logger); err != nil {
		return fmt.Errorf("db migration failed: %w", err)
	}

	txManager := persistence.NewTxManager(pool)
	repo := persistence.NewAccountLinkStore(pool)
	idem := persistence.NewIdempotencyStore(pool)
	outbox := persistence.NewOutboxStore()

	svc := app.NewAccountLinkService(txManager, repo, idem, outbox)

	publisher, err := buildPublisher(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("event publisher setup failed: %w", err)
	}

	processor := app.NewOutboxProcessor(
		txManager,
		outbox,
		publisher,
		app.WithOutboxProcessorBatchSize(cfg.OutboxPollBatch),
		app.WithOutboxProcessorPollDelay(cfg.OutboxPollDelay),
		app.WithOutboxProcessorLogger(logger),
	)
	go processor.Start(ctx)

	apiHandler := api.NewHandler(svc)
	httpServer := server.New(":"+cfg.Port, logger, apiHandler)

	return httpServer.Run(ctx)
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
		loadOptions = append(loadOptions,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}

	client := sns.NewFromConfig(awsCfg, func(o *sns.Options) {
		if cfg.SNSEndpoint != "" {
			o.BaseEndpoint = awsv2.String(cfg.SNSEndpoint)
		}
	})

	return events.NewSNSPublisher(client, cfg.SNSTopicARN, logger), nil
}

func migrateWithRetry(
	ctx context.Context,
	pool *pgxpool.Pool,
	maxWait time.Duration,
	retryInterval time.Duration,
	logger *slog.Logger,
) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	var lastErr error
	for {
		lastErr = db.Migrate(deadlineCtx, pool)
		if lastErr == nil {
			return nil
		}

		if deadlineCtx.Err() != nil {
			return lastErr
		}

		logger.Warn("db migration attempt failed; retrying",
			"err", lastErr,
			"retry_in", retryInterval.String(),
		)

		select {
		case <-deadlineCtx.Done():
			return lastErr
		case <-time.After(retryInterval):
		}
	}
}
