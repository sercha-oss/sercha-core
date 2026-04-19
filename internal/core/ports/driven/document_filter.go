package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// DocumentIDProvider provides document IDs for pre-filtering search results.
// Implementations can apply ACL, tenant isolation, or other filtering logic.
type DocumentIDProvider interface {
	// GetAllowedDocumentIDs returns the document IDs that should be included in search.
	// Returns nil/empty slice if no filtering should be applied.
	GetAllowedDocumentIDs(ctx context.Context, query string, filters pipeline.SearchFilters) ([]string, error)
}
