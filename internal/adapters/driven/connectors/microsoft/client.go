package microsoft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"golang.org/x/time/rate"
)

// ErrResyncRequired signals that a stored delta token is no longer valid
// and the caller must restart the delta query from scratch (empty cursor).
// Microsoft Graph emits this as HTTP 410 with codes like "resyncRequired"
// or "syncStateNotFound" — surface them as a typed error so connector
// code can branch on it cleanly without string-matching.
var ErrResyncRequired = errors.New("microsoft graph: delta cursor invalidated, full resync required")

// Client provides Microsoft Graph API operations.
type Client struct {
	tokenProvider       driven.TokenProvider
	httpClient          *http.Client
	baseURL             string
	rateLimiter         *rate.Limiter
	maxRetries          int
	retry429TotalBudget time.Duration
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
	// This bounds 5xx retries (server errors, not throttling — finite retry budget is conventional).
	MaxRetries int

	// Retry429TotalBudget is the maximum cumulative wall-clock duration the
	// retry loop will spend waiting on 429 responses for a single request.
	// Microsoft Graph documents that clients should retry 429 responses
	// repeatedly using the Retry-After header until the throttle window
	// closes (https://learn.microsoft.com/en-us/graph/throttling). MaxRetries
	// is therefore the wrong shape for 429s — a wall-clock budget defends
	// against a misbehaving endpoint without artificially capping legitimate
	// throttle waits.
	//
	// Default 5 minutes covers ~99% of documented Microsoft Graph throttle
	// windows (typically 30s-2min). On exhaustion, doRequest returns a
	// typed error with the cumulative wait time and the last Retry-After
	// value so the caller can see how the budget was consumed.
	//
	// MaxRetries continues to bound 5xx retries (server bug, not throttling
	// — finite retry budget is conventional).
	Retry429TotalBudget time.Duration
}

// defaultRetry429TotalBudget is the wall-clock budget applied to consecutive
// 429 responses for a single request. Microsoft Graph documents that clients
// should retry 429 responses using Retry-After until the throttle window closes
// (https://learn.microsoft.com/en-us/graph/throttling). Five minutes covers
// ~99% of documented throttle windows (typically 30s–2min).
const defaultRetry429TotalBudget = 5 * time.Minute

// DefaultClientConfig returns the default Microsoft Graph client configuration.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:             "https://graph.microsoft.com/v1.0",
		RateLimitRPS:        10.0, // Conservative rate limit
		RequestTimeout:      30 * time.Second,
		MaxRetries:          5,
		Retry429TotalBudget: defaultRetry429TotalBudget,
	}
}

// NewClient creates a new Microsoft Graph API client.
//
// transport may be nil, in which case http.DefaultTransport is used. Production
// callers should pass connectors.SharedTransport("microsoft") so that TLS
// sessions and keepalive connections are pooled across all Microsoft connector
// instances in the process rather than each instance opening fresh connections.
//
// The *http.Client envelope (which carries the per-source timeout) is always
// allocated per-call; only the underlying transport (connection pool) is shared.
func NewClient(tokenProvider driven.TokenProvider, config *ClientConfig, transport http.RoundTripper) *Client {
	if config == nil {
		config = DefaultClientConfig()
	}
	if transport == nil {
		transport = http.DefaultTransport
	}

	budget := config.Retry429TotalBudget
	if budget <= 0 {
		budget = defaultRetry429TotalBudget
	}

	limiter := rate.NewLimiter(rate.Limit(config.RateLimitRPS), 1)

	return &Client{
		tokenProvider:       tokenProvider,
		httpClient:          &http.Client{Timeout: config.RequestTimeout, Transport: transport},
		baseURL:             strings.TrimSuffix(config.BaseURL, "/"),
		rateLimiter:         limiter,
		maxRetries:          config.MaxRetries,
		retry429TotalBudget: budget,
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

// Do implements driven.RESTClient. It is a thin export of doRequest so
// callers that hold this Client through the RESTClient port can invoke
// Graph endpoints not covered by the typed methods above while reusing the
// same auth, rate-limit, and retry behaviour.
func (c *Client) Do(ctx context.Context, method, path string, body, result any) error {
	return c.doRequest(ctx, method, path, body, result)
}

// WaitForRateLimit blocks until the next request is permitted by the
// per-client rate budget (or ctx is cancelled). Use this from external
// callers that issue HTTP requests to Microsoft endpoints outside of
// doRequest (e.g. file-content downloads via pre-signed CDN URLs) so
// every Microsoft-bound request shares the same token bucket.
func (c *Client) WaitForRateLimit(ctx context.Context) error {
	return c.rateLimiter.Wait(ctx)
}

// Retry429TotalBudget returns the per-request wall-clock budget the
// client applies to consecutive 429 responses. Exposed so external
// callers issuing requests outside doRequest (e.g. content downloads
// via pre-signed CDN URLs) can apply the same budget for consistency.
func (c *Client) Retry429TotalBudget() time.Duration {
	return c.retry429TotalBudget
}

// Compile-time assertion that *Client satisfies the RESTClient port.
var _ driven.RESTClient = (*Client)(nil)

// doRequest performs an authenticated HTTP request with rate limiting and retry logic.
//
// 429 (throttling) and 5xx (server error) are handled separately:
//   - 429: retried until the cumulative wall-clock wait exceeds c.retry429TotalBudget.
//     Per Microsoft Graph guidance (https://learn.microsoft.com/en-us/graph/throttling),
//     clients should keep retrying with the Retry-After delay until the throttle
//     window closes. MaxRetries is not the right shape for this — a wall-clock
//     budget is used instead.
//   - 5xx: bounded by c.maxRetries with per-attempt exponential backoff.
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

	var (
		resp                *http.Response
		lastRetriedStatus   int
		lastRetryAfterHeader string
		total429Wait        time.Duration
		budgetExhausted     bool
		// serverAttempt counts 5xx attempts for the exponential backoff formula
		// and the "after N attempts" exhaustion message. It is NOT incremented on
		// 429 retries so the 5xx budget is unaffected by throttle waits.
		serverAttempt int
		// retry429Count is the per-429-retry index used for the fallback backoff
		// formula when no Retry-After header is present.
		retry429Count int
	)

	for {
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

		// 429: throttling — retry using wall-clock budget, not MaxRetries.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterHeader := resp.Header.Get("Retry-After")
			backoff := ParseRetryAfter(retryAfterHeader)
			if backoff <= 0 {
				backoff = time.Duration(retry429Count+1) * time.Second
			}
			if backoff < time.Second {
				backoff = time.Second
			}

			// Check budget before committing to the sleep. If this sleep would
			// push cumulative wait past the budget, exhaust now.
			if total429Wait+backoff > c.retry429TotalBudget {
				lastRetriedStatus = resp.StatusCode
				lastRetryAfterHeader = retryAfterHeader
				budgetExhausted = true
				_ = resp.Body.Close()
				break
			}

			total429Wait += backoff
			lastRetriedStatus = resp.StatusCode
			lastRetryAfterHeader = retryAfterHeader
			retry429Count++
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// Success or non-retryable 4xx error: exit the loop.
		if resp.StatusCode < 500 {
			break
		}

		// 5xx: server error — bounded by MaxRetries, exponential backoff,
		// Retry-After honoured if present.
		if serverAttempt >= c.maxRetries {
			// Budget exhausted for 5xx — record and break.
			lastRetriedStatus = resp.StatusCode
			_ = resp.Body.Close()
			break
		}

		backoff := ParseRetryAfter(resp.Header.Get("Retry-After"))
		if backoff <= 0 {
			backoff = time.Duration(serverAttempt+1) * time.Second
		}
		lastRetriedStatus = resp.StatusCode
		serverAttempt++
		_ = resp.Body.Close()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	// Budget-exhausted on 429: the throttle window outlasted the per-request budget.
	if budgetExhausted {
		return fmt.Errorf("microsoft graph %s %s: 429 throttling persisted past %s budget (cumulative wait %s, last Retry-After=%s)",
			method, path, c.retry429TotalBudget, total429Wait, lastRetryAfterHeader)
	}

	// 5xx retries exhausted: resp.Body was closed inside the loop.
	if resp == nil || (lastRetriedStatus != 0 && resp.StatusCode >= 500) {
		return fmt.Errorf("microsoft graph %s %s: retries exhausted (last status %d after %d attempts)",
			method, path, lastRetriedStatus, c.maxRetries+1)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != nil {
			// 410 Gone on a delta endpoint indicates the stored token has
			// aged out or been invalidated — Microsoft expects the caller
			// to start a fresh delta query rather than retry the same
			// link. Map both documented codes plus the HTTP status itself
			// to ErrResyncRequired so connectors can recover without
			// string-matching the message.
			if resp.StatusCode == http.StatusGone {
				return fmt.Errorf("%w: %s - %s", ErrResyncRequired, errResp.Error.Code, errResp.Error.Message)
			}
			return fmt.Errorf("microsoft graph API error %d: %s - %s",
				resp.StatusCode, errResp.Error.Code, errResp.Error.Message)
		}
		if resp.StatusCode == http.StatusGone {
			return fmt.Errorf("%w: %s", ErrResyncRequired, string(respBody))
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

// ParseRetryAfter returns the duration to wait as advised by the HTTP
// Retry-After header. Empty header or unparsable values return 0. Only
// the delta-seconds form is supported per RFC 7231; HTTP-date form
// returns 0 (callers fall back to their own backoff strategy).
//
// Exported so callers issuing requests to Microsoft endpoints outside
// of doRequest (notably content downloads via pre-signed CDN URLs)
// can apply the same Retry-After semantics as the API path.
func ParseRetryAfter(headerValue string) time.Duration {
	if headerValue == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(headerValue)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}
