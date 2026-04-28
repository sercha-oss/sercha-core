package notion

import (
	"bytes"
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

// Client provides Notion API operations.
type Client struct {
	tokenProvider driven.TokenProvider
	httpClient    *http.Client
	baseURL       string
	version       string
	rateLimiter   *rate.Limiter
	maxRetries    int
}

// NewClient creates a new Notion API client.
func NewClient(tokenProvider driven.TokenProvider, config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	// Create rate limiter: 3 requests per second
	limiter := rate.NewLimiter(rate.Limit(config.RateLimitRPS), 1)

	return &Client{
		tokenProvider: tokenProvider,
		httpClient:    &http.Client{Timeout: config.RequestTimeout},
		baseURL:       strings.TrimSuffix(config.APIBaseURL, "/"),
		version:       config.NotionVersion,
		rateLimiter:   limiter,
		maxRetries:    config.MaxRetries,
	}
}

// Search searches for pages and databases.
// Supports filtering by last_edited_time for incremental sync.
func (c *Client) Search(ctx context.Context, filter *SearchFilter, cursor string) (*SearchResponse, error) {
	body := map[string]interface{}{
		"page_size": 100,
	}

	if filter != nil {
		body["filter"] = filter
	}

	if cursor != "" {
		body["start_cursor"] = cursor
	}

	var result SearchResponse
	if err := c.doRequest(ctx, "POST", "/search", body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SearchFilter represents a search filter.
type SearchFilter struct {
	Property string      `json:"property"` // "object" or timestamp field
	Value    interface{} `json:"value,omitempty"`
}

// GetPage retrieves a page by ID.
func (c *Client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	var page Page
	if err := c.doRequest(ctx, "GET", fmt.Sprintf("/pages/%s", pageID), nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetDatabase retrieves a database by ID.
func (c *Client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	var database Database
	if err := c.doRequest(ctx, "GET", fmt.Sprintf("/databases/%s", databaseID), nil, &database); err != nil {
		return nil, err
	}
	return &database, nil
}

// GetBlocks retrieves child blocks of a page or block.
func (c *Client) GetBlocks(ctx context.Context, blockID string, cursor string) (*BlocksResponse, error) {
	path := fmt.Sprintf("/blocks/%s/children", blockID)
	if cursor != "" {
		path += "?start_cursor=" + cursor
	}

	var result BlocksResponse
	if err := c.doRequest(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetBlocksRecursive retrieves all blocks recursively up to maxDepth.
func (c *Client) GetBlocksRecursive(ctx context.Context, blockID string, maxDepth int) ([]Block, error) {
	return c.getBlocksRecursiveImpl(ctx, blockID, 0, maxDepth)
}

func (c *Client) getBlocksRecursiveImpl(ctx context.Context, blockID string, currentDepth, maxDepth int) ([]Block, error) {
	if currentDepth >= maxDepth {
		return nil, nil
	}

	var allBlocks []Block
	cursor := ""

	for {
		resp, err := c.GetBlocks(ctx, blockID, cursor)
		if err != nil {
			return nil, err
		}

		for _, block := range resp.Results {
			allBlocks = append(allBlocks, block)

			// Recursively fetch children if block has children
			if block.HasChildren {
				children, err := c.getBlocksRecursiveImpl(ctx, block.ID, currentDepth+1, maxDepth)
				if err != nil {
					// Log error but continue with other blocks
					continue
				}
				allBlocks = append(allBlocks, children...)
			}
		}

		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}

	return allBlocks, nil
}

// QueryDatabase queries a database for pages.
func (c *Client) QueryDatabase(ctx context.Context, databaseID string, filter interface{}, cursor string) (*QueryDatabaseResponse, error) {
	body := map[string]interface{}{
		"page_size": 100,
	}

	if filter != nil {
		body["filter"] = filter
	}

	if cursor != "" {
		body["start_cursor"] = cursor
	}

	var result QueryDatabaseResponse
	if err := c.doRequest(ctx, "POST", fmt.Sprintf("/databases/%s/query", databaseID), body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetUser retrieves the authenticated user's information.
func (c *Client) GetUser(ctx context.Context) (*UserResponse, error) {
	var user UserResponse
	if err := c.doRequest(ctx, "GET", "/users/me", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Do implements driven.RESTClient. It is a thin export of doRequest so
// callers that hold this Client through the RESTClient port can invoke
// Notion endpoints not covered by the typed methods above while reusing
// the same auth, rate-limit, and retry behaviour.
func (c *Client) Do(ctx context.Context, method, path string, body, result any) error {
	return c.doRequest(ctx, method, path, body, result)
}

// Compile-time assertion that *Client satisfies the RESTClient port.
var _ driven.RESTClient = (*Client)(nil)

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
		bodyReader = bytes.NewReader(bodyJSON)
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

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Notion-Version", c.version)
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
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("notion API error %d: %s - %s", errResp.Status, errResp.Code, errResp.Message)
		}
		return fmt.Errorf("notion API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// ExtractPlainText extracts plain text from rich text array.
func ExtractPlainText(richTexts []RichText) string {
	var sb strings.Builder
	for _, rt := range richTexts {
		sb.WriteString(rt.PlainText)
	}
	return sb.String()
}

// ExtractBlockContent extracts plain text content from a block.
func ExtractBlockContent(block Block) string {
	switch block.Type {
	case "paragraph":
		if block.Paragraph != nil {
			return ExtractPlainText(block.Paragraph.RichText)
		}
	case "heading_1":
		if block.Heading1 != nil {
			return "# " + ExtractPlainText(block.Heading1.RichText)
		}
	case "heading_2":
		if block.Heading2 != nil {
			return "## " + ExtractPlainText(block.Heading2.RichText)
		}
	case "heading_3":
		if block.Heading3 != nil {
			return "### " + ExtractPlainText(block.Heading3.RichText)
		}
	case "bulleted_list_item":
		if block.BulletedListItem != nil {
			return "- " + ExtractPlainText(block.BulletedListItem.RichText)
		}
	case "numbered_list_item":
		if block.NumberedListItem != nil {
			return "1. " + ExtractPlainText(block.NumberedListItem.RichText)
		}
	case "to_do":
		if block.ToDo != nil {
			checkbox := "[ ]"
			if block.ToDo.Checked {
				checkbox = "[x]"
			}
			return checkbox + " " + ExtractPlainText(block.ToDo.RichText)
		}
	case "toggle":
		if block.Toggle != nil {
			return ExtractPlainText(block.Toggle.RichText)
		}
	case "code":
		if block.Code != nil {
			return "```" + block.Code.Language + "\n" + ExtractPlainText(block.Code.RichText) + "\n```"
		}
	case "quote":
		if block.Quote != nil {
			return "> " + ExtractPlainText(block.Quote.RichText)
		}
	case "callout":
		if block.Callout != nil {
			return ExtractPlainText(block.Callout.RichText)
		}
	case "child_page":
		if block.ChildPage != nil {
			return "[[" + block.ChildPage.Title + "]]"
		}
	case "child_database":
		if block.ChildDatabase != nil {
			return "[[Database: " + block.ChildDatabase.Title + "]]"
		}
	}
	return ""
}

// GetPageTitle extracts the title from page properties.
func GetPageTitle(properties Properties) string {
	for _, prop := range properties {
		if prop.Type == "title" {
			if title := prop.GetTitle(); len(title) > 0 {
				return ExtractPlainText(title)
			}
		}
	}
	return "Untitled"
}

// GetDatabaseTitle extracts the title from a database.
func GetDatabaseTitle(db *Database) string {
	if len(db.Title) > 0 {
		return ExtractPlainText(db.Title)
	}
	return "Untitled Database"
}
