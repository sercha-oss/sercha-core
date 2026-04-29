package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// DocumentStore handles document persistence (PostgreSQL)
type DocumentStore interface {
	// Save creates or updates a document
	Save(ctx context.Context, doc *domain.Document) error

	// SaveBatch saves multiple documents in a transaction
	SaveBatch(ctx context.Context, docs []*domain.Document) error

	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (*domain.Document, error)

	// GetByIDs retrieves multiple documents by ID in a single round trip.
	// The returned map is keyed by document ID; missing IDs simply don't
	// appear (this is not an error). Used by the search service to avoid
	// N database queries when materialising ranked search results.
	GetByIDs(ctx context.Context, ids []string) (map[string]*domain.Document, error)

	// GetByExternalID retrieves a document by source and external ID
	GetByExternalID(ctx context.Context, sourceID, externalID string) (*domain.Document, error)

	// GetBySource retrieves all documents for a source with pagination
	GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error)

	// Delete deletes a document
	Delete(ctx context.Context, id string) error

	// DeleteBySource deletes all documents for a source
	DeleteBySource(ctx context.Context, sourceID string) error

	// DeleteBySourceAndContainer deletes all documents for a specific container within a source
	DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error

	// DeleteBatch deletes multiple documents by ID
	DeleteBatch(ctx context.Context, ids []string) error

	// Count returns total document count
	Count(ctx context.Context) (int, error)

	// CountBySource returns document count for a source
	CountBySource(ctx context.Context, sourceID string) (int, error)

	// ListExternalIDs returns all external IDs for a source (for diff sync)
	ListExternalIDs(ctx context.Context, sourceID string) ([]string, error)
}
