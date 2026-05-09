package driven

import (
	"context"
	"errors"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// joinErrs collapses a slice of observer errors into a single value.
// errors.Join handles nil-input correctly (returns nil). Centralising the
// behaviour makes the composite implementations short and clear.
func joinErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

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

// CompositeDocumentDeleteObserver fans out delete events to multiple
// underlying observers. Each observer is invoked in registration order;
// observer errors are aggregated (not short-circuited) so a failure in
// one observer does not prevent later observers from running.
//
// This composite is the canonical way to attach more than one cleanup
// concern to the same delete signal — e.g. permission tuple cleanup
// alongside entity-register cleanup. Wiring callers (main.go) construct
// the composite and pass it as the single DocumentDeleteObserver to
// SourceService and SyncOrchestrator.
//
// Per Core's existing contract, observer errors are logged-and-ignored
// by the caller (sync/source service). This composite preserves that
// contract: it returns an aggregated error so the caller can log it,
// but the deletion itself is not affected by any observer failure.
type CompositeDocumentDeleteObserver struct {
	observers []DocumentDeleteObserver
}

// NewCompositeDocumentDeleteObserver constructs a fanout observer.
// Nil observers in the input are filtered out so callers can pass
// optionally-nil observers without manual nil checks.
func NewCompositeDocumentDeleteObserver(observers ...DocumentDeleteObserver) *CompositeDocumentDeleteObserver {
	filtered := make([]DocumentDeleteObserver, 0, len(observers))
	for _, o := range observers {
		if o != nil {
			filtered = append(filtered, o)
		}
	}
	return &CompositeDocumentDeleteObserver{observers: filtered}
}

// OnDocumentDeleted invokes every underlying observer and aggregates
// errors. Observers run in the order they were provided to the
// constructor; later observers are not skipped when an earlier one
// returns an error.
func (c *CompositeDocumentDeleteObserver) OnDocumentDeleted(ctx context.Context, source *domain.Source, doc *domain.Document) error {
	var errs []error
	for _, o := range c.observers {
		if err := o.OnDocumentDeleted(ctx, source, doc); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrs(errs)
}

// OnSourceDeleted is the source-cascade equivalent of OnDocumentDeleted.
// Same fan-out + aggregation semantics.
func (c *CompositeDocumentDeleteObserver) OnSourceDeleted(ctx context.Context, source *domain.Source) error {
	var errs []error
	for _, o := range c.observers {
		if err := o.OnSourceDeleted(ctx, source); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrs(errs)
}
