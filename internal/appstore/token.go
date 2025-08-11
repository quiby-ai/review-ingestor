package appstore

import (
	"context"
	"errors"
	"fmt"

	landingx "github.com/quiby-ai/common/pkg/appstore/landing"
	tokenx "github.com/quiby-ai/common/pkg/appstore/token"
	"github.com/quiby-ai/common/pkg/httpx"
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
	url, _ := landingx.BuildLandingURL(country, appName, appID)
	response, err := t.http.DoGET(ctx, url, nil, nil)
	if err != nil {
		return "", fmt.Errorf("extract token failed: %w", err)
	}
	if response.Status != 200 {
		return "", fmt.Errorf("unexpected status: %d", response.Status)
	}
	token, _, exists := tokenx.ExtractBearerToken(string(response.Body))
	if exists && token != "" {
		return token, nil
	}
	return "", ErrTokenNotFound
}
