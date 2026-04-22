package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// DocumentIDProvider provides document IDs for pre-filtering search results.
// Implementations can apply ACL, tenant isolation, or other authoritative filtering logic.
type DocumentIDProvider interface {
	// GetAllowedDocumentIDs returns the document IDs that the caller is authorised
	// to see, using a three-case contract:
	//
	//   - nil return (and nil error): the provider declines to filter. Downstream
	//     treats this as "no filter" and passes the query through unchanged.
	//
	//   - empty slice return ([]string{}, nil): authoritative deny-all. Downstream
	//     MUST return zero results. This is the critical case that distinguishes
	//     this contract from the previous fail-open design — implementations MUST
	//     return an empty slice (not nil) when asserting that the caller has access
	//     to zero documents.
	//
	//   - non-empty slice return: authoritative allow-list. Downstream filters
	//     results to exactly these IDs.
	//
	//   - non-nil error: propagated; the query is rejected. Do not return a nil
	//     slice with an error to mean "deny on uncertainty" — use the error path.
	//
	// The pipeline stage consuming this provider translates the return into a
	// *domain.DocumentIDFilter: nil → leave the filter unset; non-nil (including
	// empty) → &DocumentIDFilter{Apply: true, IDs: <return>}.
	GetAllowedDocumentIDs(ctx context.Context, query string, filters pipeline.SearchFilters) ([]string, error)
}
