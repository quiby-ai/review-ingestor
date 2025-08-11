package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/quiby-ai/review-ingestor/config"
)

func InitPostgres(cfg config.PostgresConfig) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := pingDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return db, nil
}

func pingDatabase(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func migrateSchema(db *sql.DB) error {
	const schema = `
	CREATE TABLE IF NOT EXISTS raw_reviews (
		id TEXT PRIMARY KEY,
		app_id TEXT NOT NULL,
		country VARCHAR(2) NOT NULL,
		rating SMALLINT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		reviewed_at TIMESTAMPTZ NOT NULL,
		response_date TIMESTAMPTZ,
		response_content TEXT
	);`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema migration: %w", err)
	}
	return nil
}

type ReviewRepository struct {
	db *sql.DB
}

func NewReviewRepository(db *sql.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) SaveRawReview(ctx context.Context, id, appID, country string, rating int, title, content string, reviewedAt time.Time, responseDate *time.Time, responseContent *string) error {
	const query = `
		INSERT INTO raw_reviews (id, app_id, country, rating, title, content, reviewed_at, response_date, response_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO NOTHING;`
	_, err := r.db.ExecContext(ctx, query, id, appID, country, rating, title, content, reviewedAt, responseDate, responseContent)
	return err
}
