package microsoft

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"golang.org/x/time/rate"
)

// Client provides Microsoft Graph API operations.
type Client struct {
	tokenProvider driven.TokenProvider
	httpClient    *http.Client
	baseURL       string
	rateLimiter   *rate.Limiter
	maxRetries    int
}

// ClientConfig contains configuration for the Microsoft Graph client.
type ClientConfig struct {
	// BaseURL is the base URL for Microsoft Graph API.
	// Defaults to https://graph.microsoft.com/v1.0
	BaseURL string

	// RateLimitRPS is the rate limit in requests per second.
	// Microsoft Graph has different rate limits per endpoint, but we use a conservative default.
	RateLimitRPS float64

	// RequestTimeout is the timeout for API requests.
	RequestTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts for failed requests.
	MaxRetries int
}

// DefaultClientConfig returns the default Microsoft Graph client configuration.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   10.0, // Conservative rate limit
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
}

// NewClient creates a new Microsoft Graph API client.
func NewClient(tokenProvider driven.TokenProvider, config *ClientConfig) *Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	limiter := rate.NewLimiter(rate.Limit(config.RateLimitRPS), 1)

	return &Client{
		tokenProvider: tokenProvider,
		httpClient:    &http.Client{Timeout: config.RequestTimeout},
		baseURL:       strings.TrimSuffix(config.BaseURL, "/"),
		rateLimiter:   limiter,
		maxRetries:    config.MaxRetries,
	}
}

// GetMe retrieves the authenticated user's information.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.doRequest(ctx, "GET", "/me", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetDelta retrieves changes in a drive using delta queries.
// If deltaLink is empty, starts a new delta query. Otherwise, uses the provided delta link.
func (c *Client) GetDelta(ctx context.Context, deltaLink string) (*DeltaResponse, error) {
	var path string
	if deltaLink != "" {
		// deltaLink is a full URL, extract the path
		path = strings.TrimPrefix(deltaLink, c.baseURL)
	} else {
		// Start new delta query
		path = "/me/drive/root/delta"
	}

	var result DeltaResponse
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDriveItems retrieves items from a drive or folder.
func (c *Client) GetDriveItems(ctx context.Context, itemID string) (*DriveItemsResponse, error) {
	path := "/me/drive/items/" + itemID + "/children"

	var result DriveItemsResponse
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDriveItem retrieves a specific drive item.
func (c *Client) GetDriveItem(ctx context.Context, itemID string) (*DriveItem, error) {
	path := "/me/drive/items/" + itemID

	var item DriveItem
	if err := c.doRequest(ctx, "GET", path, nil, &item); err != nil {
		return nil, err
	}

	return &item, nil
}

// GetDriveItemContent downloads the content of a file.
// Returns the content as a byte slice.
func (c *Client) GetDriveItemContent(ctx context.Context, itemID string) ([]byte, error) {
	token, err := c.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	path := fmt.Sprintf("%s/me/drive/items/%s/content", c.baseURL, itemID)

	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get content failed (%d): %s", resp.StatusCode, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return content, nil
}

// GetNextPage retrieves the next page of results using a nextLink URL.
func (c *Client) GetNextPage(ctx context.Context, nextLink string, result interface{}) error {
	// nextLink is a full URL, extract the path
	path := strings.TrimPrefix(nextLink, c.baseURL)
	return c.doRequest(ctx, "GET", path, nil, result)
}

// doRequest performs an authenticated HTTP request with rate limiting and retry logic.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	token, err := c.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyJSON))
	}

	var resp *http.Response
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Wait for rate limiter
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter: %w", err)
		}

		// Reset body reader if retrying
		if bodyReader != nil {
			if seeker, ok := bodyReader.(io.Seeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("reset body reader: %w", err)
				}
			}
		}

		// Construct full URL
		fullURL := c.baseURL + path

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("do request: %w", err)
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			// Exponential backoff
			backoff := time.Duration(attempt+1) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// Success or non-retryable error
		if resp.StatusCode < 500 {
			break
		}

		// Server error - retry with exponential backoff
		_ = resp.Body.Close()
		backoff := time.Duration(attempt+1) * time.Second
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != nil {
			return fmt.Errorf("microsoft graph API error %d: %s - %s",
				resp.StatusCode, errResp.Error.Code, errResp.Error.Message)
		}
		return fmt.Errorf("microsoft graph API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
