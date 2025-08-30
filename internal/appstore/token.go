package appstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	landingx "github.com/quiby-ai/common/pkg/appstore/landing"
	tokenx "github.com/quiby-ai/common/pkg/appstore/token"
	"github.com/quiby-ai/common/pkg/httpx"
	"github.com/quiby-ai/review-ingestor/internal/logger"
)

var (
	ErrTokenNotFound = errors.New("token not found")
)

type TokenExtractor struct {
	http httpx.Client
}

func NewTokenExtractor(http httpx.Client) *TokenExtractor {
	return &TokenExtractor{http: http}
}

func (t *TokenExtractor) ExtractToken(ctx context.Context, country, appName, appID string) (string, error) {
	timer := logger.StartTimer()

	logger.Debug(ctx, "Extracting token from App Store", "country", country, "app_name", appName)

	url, _ := landingx.BuildLandingURL(country, appName, appID)
	response, err := t.http.DoGET(ctx, url, nil, nil)
	if err != nil {
		logger.LogEventWithLatency(ctx, "appstore.token.extracted", "failed", timer(), "country", country, "error", "http_request_failed")
		return "", fmt.Errorf("extract token failed: %w", err)
	}

	if response.Status != http.StatusOK {
		logger.LogEventWithLatency(ctx, "appstore.token.extracted", "failed", timer(), "country", country, "status", response.Status)
		return "", fmt.Errorf("unexpected status: %d", response.Status)
	}

	token, _, exists := tokenx.ExtractBearerToken(string(response.Body))
	if exists && token != "" {
		logger.LogEventWithLatency(ctx, "appstore.token.extracted", "success", timer(), "country", country)
		return token, nil
	}

	logger.LogEventWithLatency(ctx, "appstore.token.extracted", "failed", timer(), "country", country, "error", "token_not_found")
	return "", ErrTokenNotFound
}
