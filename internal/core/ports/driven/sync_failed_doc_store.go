package driven

import (
	"context"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// SyncFailedDocStore persists per-document failure state for the sync
// orchestrator's skip-list / retry mechanism. The orchestrator no longer
// stalls cursor-advance on per-doc errors; instead it records failures
// here and retries them on subsequent runs with exponential backoff.
//
// All methods are tenant-isolated by source_id (sources are themselves
// scoped to a team).
type SyncFailedDocStore interface {
	// Record creates or updates the skip-list row for (source_id,
	// external_id). The implementation is responsible for computing the
	// next attempt count, the next_retry_after timestamp, and whether the
	// row should be marked terminal — the caller passes the failure
	// detail and a backoff policy and the implementation does the math.
	//
	// Callers MUST pass a non-empty external_id and a non-nil failure.
	// last_error is truncated to a sane length by the implementation.
	Record(ctx context.Context, failure SyncFailedDocRecord) error

	// MarkSucceeded removes the skip-list row for (source_id, external_id)
	// after a successful re-ingest. No-op when no row exists.
	MarkSucceeded(ctx context.Context, sourceID, externalID string) error

	// ListReadyForRetry returns the skip-list rows that are due for retry
	// for the given source. "Due" means non-terminal AND
	// next_retry_after <= now. Results are bounded by limit so a source
	// with a huge backlog doesn't dominate a single sync run.
	//
	// Ordered by next_retry_after ASC so the oldest-due rows retry first.
	ListReadyForRetry(ctx context.Context, sourceID string, now time.Time, limit int) ([]domain.SyncFailedDoc, error)

	// ListBySource returns every skip-list row for a source, regardless
	// of retry status. Used by the admin endpoint that surfaces failing
	// docs for operator triage. Ordered by last_attempted_at DESC.
	ListBySource(ctx context.Context, sourceID string, limit int) ([]domain.SyncFailedDoc, error)

	// CountBySource returns the number of skip-list rows for a source.
	// Used to drive small UI badges ("12 failing docs") without paging.
	CountBySource(ctx context.Context, sourceID string) (int, error)
}

// SyncFailedDocRecord is the input to Record. The implementation derives
// the resulting attempt_count, next_retry_after, and terminal flag from
// the supplied policy plus any prior row's state.
type SyncFailedDocRecord struct {
	SourceID   string
	ExternalID string
	// Err is the error returned by the per-doc processing path.
	// Stringified and truncated by the store; pass it through as-is.
	Err error
	// Now is the wall-clock time the failure happened — passed in so
	// tests can use a fake clock.
	Now time.Time
	// Backoff is the schedule the store applies when computing
	// next_retry_after and the terminal flag. Required.
	Backoff RetryBackoff
}

// RetryBackoff describes the per-source retry policy. The store applies
// the schedule deterministically so two callers with the same policy
// produce the same next_retry_after.
//
// For attempt N (1-indexed), the next-retry delay is:
//
//	delay = min(base * 2^(N-1), max)
//
// Once N exceeds MaxAttempts the row is marked terminal — no further
// retry pickups, but the row stays for operator visibility.
type RetryBackoff struct {
	Base        time.Duration
	Max         time.Duration
	MaxAttempts int
}
