package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// DocumentIngestObserver is invoked once per document after successful ingestion.
// Implementations may perform arbitrary post-processing: secondary indexing, cache
// warming, audit writes, etc.
//
// Contract:
//   - Called synchronously after documentStore.Save succeeds.
//   - Called once per successfully-saved document.
//   - Not called when documentStore.Save fails.
//   - Returned errors are logged and ignored — observer failure does NOT fail the
//     ingest. This matches the nil-guarded log-and-continue pattern already used
//     for SyncEventRepository in SyncOrchestrator.
type DocumentIngestObserver interface {
	OnDocumentIngested(ctx context.Context, source *domain.Source, doc *domain.Document) error
}
