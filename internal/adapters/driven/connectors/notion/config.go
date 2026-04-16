package notion

import "time"

// Config contains configuration for the Notion connector.
type Config struct {
	// APIBaseURL is the base URL for Notion API.
	// Defaults to https://api.notion.com/v1
	APIBaseURL string

	// NotionVersion is the Notion API version header.
	// Must be set to "2022-06-28" for all requests.
	NotionVersion string

	// RateLimitRPS is the rate limit in requests per second.
	// Notion allows 3 requests per second per integration.
	RateLimitRPS float64

	// RequestTimeout is the timeout for API requests.
	RequestTimeout time.Duration

	// MaxBlockDepth is the maximum recursion depth for nested blocks.
	// Prevents infinite recursion when extracting block content.
	MaxBlockDepth int

	// MaxRetries is the maximum number of retry attempts for failed requests.
	MaxRetries int
}

// DefaultConfig returns the default Notion connector configuration.
func DefaultConfig() *Config {
	return &Config{
		APIBaseURL:     "https://api.notion.com/v1",
		NotionVersion:  "2022-06-28",
		RateLimitRPS:   3.0, // 3 requests per second
		RequestTimeout: 30 * time.Second,
		MaxBlockDepth:  10, // Prevent infinite recursion
		MaxRetries:     3,
	}
}
