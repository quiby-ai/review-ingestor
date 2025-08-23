package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/internal/appstore"
	"github.com/quiby-ai/review-ingestor/internal/producer"
	"github.com/quiby-ai/review-ingestor/internal/storage"
)

var (
	Limit int = 500
)

// Interfaces for dependency injection and testing
type TokenExtractor interface {
	ExtractToken(ctx context.Context, country, appName, appID string) (string, error)
}

type ReviewFetcher interface {
	SetToken(token string)
	FetchAllReviews(ctx context.Context, country, appID string, opts *appstore.FetchOptions) ([]appstore.Review, error)
}

type ReviewRepository interface {
	SaveRawReview(ctx context.Context, id, appID, country string, rating int, title, content string, reviewedAt time.Time, responseDate *time.Time, responseContent *string) error
}

type KafkaProducer interface {
	PublishEvent(ctx context.Context, key []byte, envelope events.Envelope[any]) error
	BuildEnvelope(event events.ExtractCompleted, sagaID string) events.Envelope[any]
}

type IngestService struct {
	extractor TokenExtractor
	fetcher   ReviewFetcher
	repo      ReviewRepository
	producer  KafkaProducer
}

func NewIngestService(te *appstore.TokenExtractor, rf *appstore.ReviewFetcher, repo *storage.ReviewRepository, prod *producer.Producer) *IngestService {
	return &IngestService{extractor: te, fetcher: rf, repo: repo, producer: prod}
}

func (s *IngestService) Process(ctx context.Context, payload []byte) error {
	var fullMessage struct {
		SagaID  string                `json:"saga_id"`
		Payload events.ExtractRequest `json:"payload"`
	}
	if err := json.Unmarshal(payload, &fullMessage); err != nil {
		return fmt.Errorf("failed to parse full message: %w", err)
	}
	inputEvent := fullMessage.Payload
	sagaID := fullMessage.SagaID

	if err := inputEvent.Validate(); err != nil {
		return fmt.Errorf("invalid incoming event: %w", err)
	}

	totalCount := 0

	tokenCountry := inputEvent.Countries[0]
	token, err := s.extractor.ExtractToken(ctx, tokenCountry, inputEvent.AppName, inputEvent.AppID)
	if err != nil {
		return fmt.Errorf("failed to extract token for country %s: %w", tokenCountry, err)
	}
	fmt.Printf("got token: %s\n", token)

	s.fetcher.SetToken(token)

	for _, country := range inputEvent.Countries {
		count, err := s.handleReviewsByCountry(ctx, inputEvent, country, Limit)
		if err != nil {
			return fmt.Errorf("failed to process country %s: %w", country, err)
		}
		totalCount += count
	}

	outputEvent := events.ExtractCompleted{
		ExtractRequest: inputEvent,
		Count:          totalCount,
	}
	if err := s.publishEvent(ctx, outputEvent, sagaID); err != nil {
		return fmt.Errorf("failed to publish prepare reviews event: %w", err)
	}

	log.Printf("Successfully processed event -> fetched %d reviews", totalCount)
	return nil
}

func (s *IngestService) handleReviewsByCountry(ctx context.Context, event events.ExtractRequest, country string, maxLimit int) (int, error) {
	log.Printf("Processing country: %s for app: %s", country, event.AppID)

	afterDate, _ := time.Parse("2006-01-02", event.DateFrom)
	opts := &appstore.FetchOptions{
		Limit:    20,
		Offset:   0,
		After:    &afterDate,
		MaxLimit: maxLimit,
	}

	reviews, err := s.fetcher.FetchAllReviews(ctx, country, event.AppID, opts)
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
			event.AppID,
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

func (s *IngestService) publishEvent(ctx context.Context, event events.ExtractCompleted, sagaID string) error {
	envelope := s.producer.BuildEnvelope(event, sagaID)
	return s.producer.PublishEvent(ctx, []byte(sagaID), envelope)
}
