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
//   - Called once per successfully-saved document.
//   - Not called when documentStore.Save fails.
//   - Invoked asynchronously: the orchestrator dispatches the call from a
//     bounded goroutine pool and returns from the per-doc processing path
//     before the observer completes. Implementations MUST be goroutine-safe
//     because the same observer may be called concurrently from multiple
//     goroutines (one per in-flight document) and may run after the sync
//     function has returned.
//   - The ctx passed to OnDocumentIngested is detached from the orchestrator's
//     request context and bounded by SyncOrchestratorConfig.
//     OnDocumentIngestedTimeout (default 30s). Observers honouring the context
//     will be cancelled on timeout; observers that ignore it run to completion
//     without affecting sync correctness.
//   - Returned errors are logged and ignored — observer failure does NOT fail
//     the ingest. This matches the nil-guarded log-and-continue pattern used
//     by other lifecycle observers.
//   - Callers that need to wait for in-flight observers (tests, graceful
//     shutdown) call SyncOrchestrator.WaitForObservers.
type DocumentIngestObserver interface {
	OnDocumentIngested(ctx context.Context, source *domain.Source, doc *domain.Document) error
}
