package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quiby-ai/review-ingestor/internal/appstore"
	"github.com/quiby-ai/review-ingestor/internal/producer"
	"github.com/quiby-ai/review-ingestor/internal/storage"
)

// Interfaces for dependency injection and testing
type TokenExtractor interface {
	ExtractToken(ctx context.Context, country, platform, appID string) (string, error)
}

type ReviewFetcher interface {
	SetToken(token string)
	FetchAllReviews(ctx context.Context, country, appID string, opts *appstore.FetchOptions) ([]appstore.Review, error)
}

type ReviewRepository interface {
	SaveRawReview(ctx context.Context, id, appID, country string, rating int, title, content string, reviewedAt time.Time, responseDate *time.Time, responseContent *string) error
}

type KafkaProducer interface {
	Publish(ctx context.Context, payload []byte) error
}

type FetchRequest struct {
	RequestID string   `json:"request_id"`
	AppID     string   `json:"app_id"`
	AppName   string   `json:"app_name"`
	Countries []string `json:"countries"`
	DateFrom  string   `json:"date_from"`
	DateTo    string   `json:"date_to"`
	Limit     int      `json:"limit"`
}

type PrepareReviewsEvent struct {
	RequestID string `json:"request_id"`
	Count     int    `json:"count"`
}

type IngestService struct {
	extractor TokenExtractor
	fetcher   ReviewFetcher
	repo      ReviewRepository
	producer  KafkaProducer
}

func NewIngestService(te *appstore.TokenExtractor, rf *appstore.ReviewFetcher, repo *storage.ReviewRepository, prod *producer.KafkaProducer) *IngestService {
	return &IngestService{extractor: te, fetcher: rf, repo: repo, producer: prod}
}

func (s *IngestService) Process(ctx context.Context, payload []byte) error {
	var req FetchRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to parse request: %w", err)
	}

	if err := s.validateRequest(req); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}

	totalCount := 0

	tokenCountry := req.Countries[0]
	token, err := s.extractor.ExtractToken(ctx, tokenCountry, req.AppName, req.AppID)
	if err != nil {
		return fmt.Errorf("failed to extract token for country %s: %w", tokenCountry, err)
	}

	s.fetcher.SetToken(token)

	for _, country := range req.Countries {
		count, err := s.handleReviewsByCountry(ctx, req, country, req.Limit)
		if err != nil {
			return fmt.Errorf("failed to process country %s: %w", country, err)
		}
		totalCount += count
	}

	prepareEvent := PrepareReviewsEvent{
		RequestID: req.RequestID,
		Count:     totalCount,
	}
	if err := s.publishEvent(ctx, prepareEvent); err != nil {
		return fmt.Errorf("failed to publish prepare reviews event: %w", err)
	}

	log.Printf("Successfully processed request %s: fetched %d reviews", req.RequestID, totalCount)
	return nil
}

func (s *IngestService) validateRequest(req FetchRequest) error {
	if req.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if req.AppID == "" {
		return fmt.Errorf("app_id is required")
	}
	if len(req.Countries) == 0 {
		return fmt.Errorf("at least one country is required")
	}
	return nil
}

func (s *IngestService) handleReviewsByCountry(ctx context.Context, req FetchRequest, country string, maxLimit int) (int, error) {
	log.Printf("Processing country: %s for app: %s", country, req.AppID)

	var afterDate *time.Time
	if req.DateFrom != "" {
		if parsed, err := time.Parse("2006-01-02", req.DateFrom); err == nil {
			afterDate = &parsed
		}
	}

	opts := &appstore.FetchOptions{
		Limit:    20,
		Offset:   0,
		After:    afterDate,
		MaxLimit: maxLimit,
	}

	reviews, err := s.fetcher.FetchAllReviews(ctx, country, req.AppID, opts)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch reviews for country %s: %w", country, err)
	}

	for _, review := range reviews {
		reviewDate, err := time.Parse("2006-01-02T15:04:05Z", review.Attributes.Date)
		if err != nil {
			log.Printf("Failed to parse review date: %v", err)
			continue
		}

		var responseDate *time.Time
		var responseContent *string
		if review.Attributes.DeveloperResponse != nil {
			if parsed, err := time.Parse("2006-01-02T15:04:05Z", review.Attributes.DeveloperResponse.Modified); err == nil {
				responseDate = &parsed
			}
			responseContent = &review.Attributes.DeveloperResponse.Body
		}

		if err := s.repo.SaveRawReview(
			ctx,
			review.ID,
			req.AppID,
			country,
			review.Attributes.Rating,
			review.Attributes.Title,
			review.Attributes.Review,
			reviewDate,
			responseDate,
			responseContent,
		); err != nil {
			log.Printf("Failed to save review %s: %v", review.ID, err)
		}
	}

	log.Printf("Fetched and stored %d reviews for country %s", len(reviews), country)
	return len(reviews), nil
}

func (s *IngestService) publishEvent(ctx context.Context, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return s.producer.Publish(ctx, data)
}
