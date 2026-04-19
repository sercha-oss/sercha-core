package onedrive

import "time"

// Config contains configuration for the OneDrive connector.
type Config struct {
	// RateLimitRPS is the rate limit in requests per second.
	RateLimitRPS float64

	// RequestTimeout is the timeout for API requests.
	RequestTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts for failed requests.
	MaxRetries int

	// MaxFileSize is the maximum file size to download (in bytes).
	// Files larger than this will be skipped.
	MaxFileSize int64
}

// DefaultConfig returns the default OneDrive connector configuration.
func DefaultConfig() *Config {
	return &Config{
		RateLimitRPS:   10.0, // Conservative rate limit for Microsoft Graph
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024, // 100 MB
	}
}
