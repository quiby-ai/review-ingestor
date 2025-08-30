package service

import (
	"context"
	"fmt"
	"time"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/internal/appstore"
	"github.com/quiby-ai/review-ingestor/internal/logger"
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

func (s *IngestService) Handle(ctx context.Context, evt events.ExtractRequest, sagaID string) error {
	timer := logger.StartTimer()

	logger.LogEvent(ctx, "service.ingest.started", "in_progress", "countries", len(evt.Countries))

	if err := evt.Validate(); err != nil {
		logger.LogEventWithLatency(ctx, "service.ingest.completed", "failed", timer(), "error", "validation_failed")
		return fmt.Errorf("invalid incoming event: %w", err)
	}

	totalCount := 0

	tokenCountry := evt.Countries[0]
	tokenTimer := logger.StartTimer()
	token, err := s.extractor.ExtractToken(ctx, tokenCountry, evt.AppName, evt.AppID)
	if err != nil {
		logger.LogEventWithLatency(ctx, "service.token.extracted", "failed", tokenTimer(), "country", tokenCountry)
		logger.LogEventWithLatency(ctx, "service.ingest.completed", "failed", timer(), "error", "token_extraction_failed")
		return fmt.Errorf("failed to extract token for country %s: %w", tokenCountry, err)
	}
	logger.LogEventWithLatency(ctx, "service.token.extracted", "success", tokenTimer(), "country", tokenCountry)

	s.fetcher.SetToken(token)

	for _, country := range evt.Countries {
		countryTimer := logger.StartTimer()
		count, err := s.handleReviewsByCountry(ctx, evt, country, Limit)
		if err != nil {
			logger.LogEventWithLatency(ctx, "service.country.processed", "failed", countryTimer(), "country", country)
			logger.LogEventWithLatency(ctx, "service.ingest.completed", "failed", timer(), "error", "country_processing_failed")
			return fmt.Errorf("failed to process country %s: %w", country, err)
		}
		logger.LogEventWithLatency(ctx, "service.country.processed", "success", countryTimer(), "country", country, "reviews_count", count)
		totalCount += count
	}

	publishTimer := logger.StartTimer()
	outputEvent := events.ExtractCompleted{
		ExtractRequest: evt,
		Count:          totalCount,
	}
	if err := s.publishEvent(ctx, outputEvent, sagaID); err != nil {
		logger.LogEventWithLatency(ctx, "producer.event.published", "failed", publishTimer())
		logger.LogEventWithLatency(ctx, "service.ingest.completed", "failed", timer(), "error", "event_publish_failed")
		return fmt.Errorf("failed to publish prepare reviews event: %w", err)
	}
	logger.LogEventWithLatency(ctx, "producer.event.published", "success", publishTimer())

	logger.LogEventWithLatency(ctx, "service.ingest.completed", "success", timer(), "total_reviews", totalCount)
	return nil
}

func (s *IngestService) handleReviewsByCountry(ctx context.Context, event events.ExtractRequest, country string, maxLimit int) (int, error) {
	logger.Debug(ctx, "Processing country", "country", country, "app_id", event.AppID)

	afterDate, _ := time.Parse("2006-01-02", event.DateFrom)
	opts := &appstore.FetchOptions{
		Limit:    20,
		Offset:   0,
		After:    &afterDate,
		MaxLimit: maxLimit,
	}

	fetchTimer := logger.StartTimer()
	reviews, err := s.fetcher.FetchAllReviews(ctx, country, event.AppID, opts)
	if err != nil {
		logger.LogEventWithLatency(ctx, "service.reviews.fetched", "failed", fetchTimer(), "country", country)
		return 0, fmt.Errorf("failed to fetch reviews for country %s: %w", country, err)
	}
	logger.LogEventWithLatency(ctx, "service.reviews.fetched", "success", fetchTimer(), "country", country, "count", len(reviews))

	successCount := 0
	for _, review := range reviews {
		reviewCtx := logger.WithReviewID(ctx, review.ID)

		reviewDate, err := time.Parse("2006-01-02T15:04:05Z", review.Attributes.Date)
		if err != nil {
			logger.Warn(reviewCtx, "Failed to parse review date", "error", err.Error())
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

		saveTimer := logger.StartTimer()
		if err := s.repo.SaveRawReview(
			reviewCtx,
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
			logger.LogEventWithLatency(reviewCtx, "storage.review.saved", "failed", saveTimer(), "country", country)
		} else {
			logger.LogEventWithLatency(reviewCtx, "storage.review.saved", "success", saveTimer(), "country", country)
			successCount++
		}
	}

	logger.Info(ctx, "Country processing completed", "country", country, "fetched", len(reviews), "saved", successCount)
	return len(reviews), nil
}

func (s *IngestService) publishEvent(ctx context.Context, event events.ExtractCompleted, sagaID string) error {
	envelope := s.producer.BuildEnvelope(event, sagaID)
	return s.producer.PublishEvent(ctx, []byte(sagaID), envelope)
}
