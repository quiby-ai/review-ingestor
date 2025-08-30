// cmd/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/quiby-ai/common/pkg/httpx"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/quiby-ai/review-ingestor/internal/appstore"
	"github.com/quiby-ai/review-ingestor/internal/consumer"
	"github.com/quiby-ai/review-ingestor/internal/logger"
	"github.com/quiby-ai/review-ingestor/internal/producer"
	"github.com/quiby-ai/review-ingestor/internal/service"
	"github.com/quiby-ai/review-ingestor/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	logger.InitLogger(cfg.Logging)
	ctx = logger.WithTraceID(ctx, "")

	logger.Info(ctx, "Starting review ingestor service", "version", "1.0.0")

	deps, err := initializeDependencies(cfg)
	if err != nil {
		logger.Error(ctx, "Failed to initialize dependencies", err)
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}
	defer deps.cleanup(ctx)

	logger.LogEvent(ctx, "app.startup", "success")

	if err := deps.consumer.Run(ctx); err != nil {
		logger.Error(ctx, "Consumer exited with error", err)
		return fmt.Errorf("consumer exited with error: %w", err)
	}

	logger.LogEvent(ctx, "app.shutdown", "success")
	return nil
}

type dependencies struct {
	db       *sql.DB
	consumer *consumer.KafkaConsumer
	producer *producer.Producer
}

func (d *dependencies) cleanup(ctx context.Context) {
	if d.db != nil {
		logger.Debug(ctx, "Closing database connection")
		if err := d.db.Close(); err != nil {
			logger.Error(ctx, "Error closing database", err)
		}
	}
	if d.consumer != nil {
		logger.Debug(ctx, "Closing Kafka consumer")
		if err := d.consumer.Close(); err != nil {
			logger.Error(ctx, "Error closing Kafka consumer", err)
		}
	}
	if d.producer != nil {
		logger.Debug(ctx, "Closing Kafka producer")
		if err := d.producer.Close(); err != nil {
			logger.Error(ctx, "Error closing Kafka producer", err)
		}
	}
}

func initializeDependencies(cfg *config.Config) (*dependencies, error) {
	httpClient := httpx.New(httpx.Config{
		Timeout:        cfg.HTTP.Timeout,
		MaxRetries:     cfg.HTTP.MaxRetries,
		BackoffInitial: cfg.HTTP.BackoffInitial,
		BackoffMax:     cfg.HTTP.BackoffMax,
		UserAgents:     cfg.HTTP.UserAgents,
	})

	db, err := storage.InitPostgres(cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	tokenExtractor := appstore.NewTokenExtractor(httpClient)
	reviewFetcher := appstore.NewReviewFetcher(httpClient, "", *cfg)

	repo := storage.NewReviewRepository(db)

	prod := producer.NewProducer(cfg.Kafka)

	svc := service.NewIngestService(tokenExtractor, reviewFetcher, repo, prod)

	consumer := consumer.NewKafkaConsumer(cfg.Kafka, svc)

	return &dependencies{
		db:       db,
		consumer: consumer,
		producer: prod,
	}, nil
}
