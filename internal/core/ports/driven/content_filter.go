package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// ContentFilter determines whether content should be fetched and processed based on
// file path patterns and MIME type exclusions. This port enables early filtering
// before content is fetched, reducing wasted resources.
type ContentFilter interface {
	// ShouldFetchContent determines if content should be fetched for a file.
	// It combines path pattern matching and MIME type detection to make the decision.
	// Returns:
	//   - shouldFetch: true if content should be fetched and processed
	//   - mimeType: detected MIME type from the file path
	ShouldFetchContent(ctx context.Context, path string, settings *domain.SyncExclusionSettings) (shouldFetch bool, mimeType string)

	// GetMimeType returns the MIME type for a file path based on its extension or filename.
	// This provides a single source of truth for MIME type detection across the application.
	GetMimeType(path string) string
}
