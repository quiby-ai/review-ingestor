package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/quiby-ai/review-ingestor/config"

	"github.com/quiby-ai/common/pkg/httpx"
)

type Review struct {
	ID         string           `json:"id"`
	Attributes ReviewAttributes `json:"attributes"`
}

type ReviewAttributes struct {
	Date              string             `json:"date"`
	Rating            int                `json:"rating"`
	Review            string             `json:"review"`
	Title             string             `json:"title"`
	DeveloperResponse *DeveloperResponse `json:"developerResponse,omitempty"`
}

type DeveloperResponse struct {
	Body     string `json:"body"`
	Modified string `json:"modified"`
}

type ReviewsResponse struct {
	Next string   `json:"next,omitempty"`
	Data []Review `json:"data"`
}

type FetchOptions struct {
	Limit    int
	Offset   int
	After    *time.Time
	MaxLimit int
	Sleep    *time.Duration
}

type ReviewFetcher struct {
	http        httpx.Client
	token       string
	appStoreCfg config.AppStoreConfig
	httpCfg     config.HTTPConfig
}

func NewReviewFetcher(http httpx.Client, token string, cfg config.Config) *ReviewFetcher {
	return &ReviewFetcher{http: http, token: token, appStoreCfg: cfg.AppStore, httpCfg: cfg.HTTP}
}

func (r *ReviewFetcher) SetToken(token string) {
	r.token = token
}

func (r *ReviewFetcher) FetchReviews(ctx context.Context, country, appID string, opts *FetchOptions) (*ReviewsResponse, error) {
	if opts == nil {
		opts = &FetchOptions{
			Limit:  20,
			Offset: 0,
		}
	}

	requestURL, headers := r.prepareQuery(country, appID, opts)

	response, err := r.http.DoGET(ctx, requestURL, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviews: %w", err)
	}

	if response.Status == 404 {
		return nil, fmt.Errorf("app not found or not available in country %s", country)
	}

	if response.Status != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", response.Status)
	}

	var reviewsResp ReviewsResponse
	if err := json.Unmarshal(response.Body, &reviewsResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &reviewsResp, nil
}

func (r *ReviewFetcher) FetchAllReviews(ctx context.Context, country string, appID string, opts *FetchOptions) ([]Review, error) {
	if opts == nil {
		opts = &FetchOptions{
			Limit:  20,
			Offset: 0,
		}
	}

	var allReviews []Review
	fetchedCount := 0
	currentOffset := opts.Offset

	backoffDelay := 1 * time.Second
	maxBackoffDelay := 60 * time.Second
	maxRetries := 5
	currentRetries := 0

	for {
		select {
		case <-ctx.Done():
			return allReviews, ctx.Err()
		default:
		}

		currentOpts := &FetchOptions{
			Limit:    opts.Limit,
			Offset:   currentOffset,
			After:    opts.After,
			MaxLimit: opts.MaxLimit,
			Sleep:    opts.Sleep,
		}

		reviewsResp, err := r.FetchReviews(ctx, country, appID, currentOpts)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "429") || strings.Contains(strings.ToLower(err.Error()), "too many") {
				if currentRetries >= maxRetries {
					return allReviews, fmt.Errorf("maximum retry attempts exceeded: %w", err)
				}

				time.Sleep(backoffDelay)
				backoffDelay = time.Duration(math.Min(float64(backoffDelay*2), float64(maxBackoffDelay)))
				currentRetries++
				continue
			}
			return allReviews, err
		}

		backoffDelay = 1 * time.Second
		currentRetries = 0

		newReviewsAdded := false
		for _, review := range reviewsResp.Data {
			reviewDate, err := time.Parse("2006-01-02T15:04:05Z", review.Attributes.Date)
			if err != nil {
				continue
			}

			if opts.After != nil && reviewDate.Before(*opts.After) {
				continue
			}

			allReviews = append(allReviews, review)
			fetchedCount++
			newReviewsAdded = true

			if fetchedCount >= opts.MaxLimit {
				return allReviews, nil
			}
		}

		if reviewsResp.Next == "" {
			break
		}

		if opts.After != nil && !newReviewsAdded {
			break
		}

		nextOffset, err := parseOffsetFromURL(reviewsResp.Next)
		if err != nil {
			break
		}
		currentOffset = nextOffset

		if opts.Sleep != nil {
			time.Sleep(*opts.Sleep)
		}
	}

	return allReviews, nil
}

func (r *ReviewFetcher) prepareQuery(country, appID string, opts *FetchOptions) (string, map[string]string) {
	baseURL := fmt.Sprintf("https://amp-api-edge.apps.apple.com/v1/catalog/%s/apps/%s/reviews", country, appID)

	params := url.Values{}
	params.Set("l", "en-GB")
	params.Set("offset", strconv.Itoa(opts.Offset))
	params.Set("sort", "recent")
	params.Set("limit", strconv.Itoa(opts.Limit))
	params.Set("platform", "web")
	params.Set("additionalPlatforms", "appletv,ipad,iphone,mac")
	params.Set("meta", "robots")

	requestURL := baseURL + "?" + params.Encode()

	headers := map[string]string{
		"accept":             "*/*",
		"accept-language":    "en-US,en;q=0.9",
		"Authorization":      fmt.Sprintf("bearer %s", r.token),
		"origin":             "https://apps.apple.com",
		"referer":            r.appStoreCfg.Referrer,
		"sec-ch-ua":          `"Not(A:Brand";v="99", "Google Chrome";v="133", "Chromium";v="133"`,
		"sec-ch-ua-mobile":   "?1",
		"sec-ch-ua-platform": `"Android"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"User-Agent":         r.httpCfg.UserAgents[rand.Intn(len(r.httpCfg.UserAgents))],
	}

	return requestURL, headers
}

func parseOffsetFromURL(urlStr string) (int, error) {
	re := regexp.MustCompile(`offset=(\d+)`)
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) < 2 {
		return 0, fmt.Errorf("offset not found in URL")
	}
	return strconv.Atoi(matches[1])
}
