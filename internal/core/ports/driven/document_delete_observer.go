package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// DocumentDeleteObserver is invoked when documents or sources are deleted.
// Implementations may perform arbitrary cleanup: side-index pruning, audit
// writes, tombstone emission, etc.
//
// This is the deletion-side mirror of DocumentIngestObserver. Same posture:
// fire after the underlying delete succeeds, log-and-continue on observer
// errors so observer health does not affect deletion correctness.
//
// Contract:
//   - OnDocumentDeleted is called once per document, after that document has
//     been removed from the document store. It fires for both upstream-driven
//     deletions (SyncOrchestrator.processDelete) and admin-driven cascades
//     (SourceService.Delete). Observers see every deletion regardless of
//     origin.
//   - OnSourceDeleted is called once after the source has been removed from
//     the source store. Per-document events for that source's documents
//     fire first (during the cascade), then OnSourceDeleted fires last.
//   - The source/document values passed in are captured before the
//     underlying delete and are safe to read inside the observer.
//   - Returned errors are logged and ignored — observer failure does NOT
//     fail the deletion. This matches the nil-guarded log-and-continue
//     pattern used by DocumentIngestObserver.
type DocumentDeleteObserver interface {
	OnDocumentDeleted(ctx context.Context, source *domain.Source, doc *domain.Document) error
	OnSourceDeleted(ctx context.Context, source *domain.Source) error
}
