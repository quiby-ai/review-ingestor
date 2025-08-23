// cmd/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/quiby-ai/common/pkg/httpx"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/quiby-ai/review-ingestor/internal/appstore"
	"github.com/quiby-ai/review-ingestor/internal/consumer"
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

	deps, err := initializeDependencies(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}
	defer deps.cleanup()

	log.Println("Starting review ingestor service...")
	if err := deps.consumer.Run(ctx); err != nil {
		return fmt.Errorf("consumer exited with error: %w", err)
	}

	log.Println("Service shutdown complete")
	return nil
}

type dependencies struct {
	db       *sql.DB
	consumer *consumer.KafkaConsumer
	producer *producer.Producer
}

func (d *dependencies) cleanup() {
	if d.db != nil {
		log.Println("Closing database connection...")
		if err := d.db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
	if d.consumer != nil {
		log.Println("Closing Kafka consumer...")
		if err := d.consumer.Close(); err != nil {
			log.Printf("Error closing Kafka consumer: %v", err)
		}
	}
	if d.producer != nil {
		log.Println("Closing Kafka producer...")
		if err := d.producer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
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
