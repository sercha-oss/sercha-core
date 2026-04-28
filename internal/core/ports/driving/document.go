package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// DocumentService provides read-only access to documents
type DocumentService interface {
	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*domain.Document, error)

	// GetContent retrieves the full content of a document
	GetContent(ctx context.Context, id string) (*domain.DocumentContent, error)

	// GetBySource retrieves all documents for a source
	GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error)

	// Count returns the total number of documents
	Count(ctx context.Context) (int, error)

	// CountBySource returns the document count for a source
	CountBySource(ctx context.Context, sourceID string) (int, error)
}
