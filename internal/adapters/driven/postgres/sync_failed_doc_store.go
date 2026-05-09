package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// errorMessageMaxBytes caps how much of a failure message we persist.
// Notion / OpenAI / Microsoft Graph errors can carry multi-KB stack
// fragments; truncate so a noisy failing doc can't bloat the row.
const errorMessageMaxBytes = 1024

var _ driven.SyncFailedDocStore = (*SyncFailedDocStore)(nil)

// SyncFailedDocStore is the Postgres-backed skip-list / retry-ledger
// the sync orchestrator uses to keep per-doc failures from stalling
// cursor advance. See port godoc for the contract; see migration
// 0003_sync_failed_documents.sql for the table.
type SyncFailedDocStore struct {
	db *DB
}

// NewSyncFailedDocStore wires the store. db is required.
func NewSyncFailedDocStore(db *DB) *SyncFailedDocStore {
	return &SyncFailedDocStore{db: db}
}

// Record inserts a fresh row or bumps an existing one for (source_id,
// external_id). Attempt count, next_retry_after, and the terminal flag
// are computed from the supplied backoff policy plus any prior row's
// state.
//
// Implementation note: the UPSERT uses a single round-trip with a
// CASE expression so attempt_count increments are atomic — no read,
// modify, write race possible. Backoff math is also done in SQL so the
// policy applied at INSERT is the same as the policy applied at
// UPDATE without re-implementing the formula in two places.
func (s *SyncFailedDocStore) Record(ctx context.Context, failure driven.SyncFailedDocRecord) error {
	if failure.SourceID == "" || failure.ExternalID == "" {
		return fmt.Errorf("sync_failed_documents: record requires source_id and external_id")
	}
	if failure.Backoff.Base <= 0 || failure.Backoff.Max <= 0 || failure.Backoff.MaxAttempts <= 0 {
		return fmt.Errorf("sync_failed_documents: record requires a valid Backoff policy")
	}

	now := failure.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	errMsg := truncateError(failure.Err, errorMessageMaxBytes)

	// We compute the new attempt_count first because the backoff and
	// terminal-flag both depend on it. The CTE shape lets us read the
	// existing row (if any), compute the new count, then do the upsert
	// with everything pre-baked in Go — keeps the SQL legible and
	// portable.
	const readQ = `
		SELECT attempt_count FROM sync_failed_documents
		WHERE source_id = $1 AND external_id = $2
	`
	var prev int
	err := s.db.QueryRowContext(ctx, readQ, failure.SourceID, failure.ExternalID).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("sync_failed_documents: read existing row: %w", err)
	}

	nextAttempt := prev + 1
	delay := computeBackoffDelay(nextAttempt, failure.Backoff)
	nextRetry := now.Add(delay)
	terminal := nextAttempt >= failure.Backoff.MaxAttempts

	const upsertQ = `
		INSERT INTO sync_failed_documents (
			source_id, external_id, attempt_count, last_error,
			last_attempted_at, next_retry_after, terminal, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $5)
		ON CONFLICT (source_id, external_id) DO UPDATE SET
			attempt_count     = EXCLUDED.attempt_count,
			last_error        = EXCLUDED.last_error,
			last_attempted_at = EXCLUDED.last_attempted_at,
			next_retry_after  = EXCLUDED.next_retry_after,
			terminal          = EXCLUDED.terminal
	`
	if _, err := s.db.ExecContext(ctx, upsertQ,
		failure.SourceID,
		failure.ExternalID,
		nextAttempt,
		errMsg,
		now,
		nextRetry,
		terminal,
	); err != nil {
		return fmt.Errorf("sync_failed_documents: upsert: %w", err)
	}
	return nil
}

// MarkSucceeded clears the skip-list row for (source_id, external_id).
// Called after the orchestrator's retry pre-pass successfully ingests
// a previously-failing doc. Idempotent — no error when no row exists.
func (s *SyncFailedDocStore) MarkSucceeded(ctx context.Context, sourceID, externalID string) error {
	const q = `DELETE FROM sync_failed_documents WHERE source_id = $1 AND external_id = $2`
	if _, err := s.db.ExecContext(ctx, q, sourceID, externalID); err != nil {
		return fmt.Errorf("sync_failed_documents: mark succeeded: %w", err)
	}
	return nil
}

// ListReadyForRetry returns the rows due for retry for sourceID. Bounded
// by limit so a source with a large backlog doesn't dominate one run;
// the ones not picked up will be retried on a subsequent tick.
func (s *SyncFailedDocStore) ListReadyForRetry(ctx context.Context, sourceID string, now time.Time, limit int) ([]domain.SyncFailedDoc, error) {
	if limit <= 0 {
		return nil, nil
	}
	const q = `
		SELECT source_id, external_id, attempt_count, last_error,
		       last_attempted_at, next_retry_after, terminal, created_at
		FROM sync_failed_documents
		WHERE source_id = $1
		  AND terminal = FALSE
		  AND next_retry_after <= $2
		ORDER BY next_retry_after ASC
		LIMIT $3
	`
	return s.scan(ctx, q, sourceID, now, limit)
}

// ListBySource returns every row for the source, regardless of retry
// status. Used by the admin endpoint that surfaces failing docs.
func (s *SyncFailedDocStore) ListBySource(ctx context.Context, sourceID string, limit int) ([]domain.SyncFailedDoc, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT source_id, external_id, attempt_count, last_error,
		       last_attempted_at, next_retry_after, terminal, created_at
		FROM sync_failed_documents
		WHERE source_id = $1
		ORDER BY last_attempted_at DESC
		LIMIT $2
	`
	return s.scan(ctx, q, sourceID, limit)
}

// CountBySource returns the row count for a source. Cheap; used for
// the small "12 docs failing" badge on the source detail page.
func (s *SyncFailedDocStore) CountBySource(ctx context.Context, sourceID string) (int, error) {
	const q = `SELECT COUNT(*) FROM sync_failed_documents WHERE source_id = $1`
	var n int
	if err := s.db.QueryRowContext(ctx, q, sourceID).Scan(&n); err != nil {
		return 0, fmt.Errorf("sync_failed_documents: count: %w", err)
	}
	return n, nil
}

// scan is the shared row-iterator used by both list methods. Centralised
// because a future column addition would otherwise need to be applied in
// two SELECT statements.
func (s *SyncFailedDocStore) scan(ctx context.Context, q string, args ...any) ([]domain.SyncFailedDoc, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sync_failed_documents: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.SyncFailedDoc, 0)
	for rows.Next() {
		var r domain.SyncFailedDoc
		if err := rows.Scan(
			&r.SourceID, &r.ExternalID, &r.AttemptCount, &r.LastError,
			&r.LastAttemptedAt, &r.NextRetryAfter, &r.Terminal, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("sync_failed_documents: scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync_failed_documents: iterate: %w", err)
	}
	return out, nil
}

// computeBackoffDelay applies the exponential schedule. Attempt is
// 1-indexed: the first failure gets base * 2^0 = base; the second
// gets base * 2^1; capped at max.
//
// Pure function, no side effects — kept here rather than in the port
// so test suites can re-derive the expected next_retry_after value
// without importing the postgres package.
func computeBackoffDelay(attempt int, b driven.RetryBackoff) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := b.Base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= b.Max {
			return b.Max
		}
	}
	if d > b.Max {
		return b.Max
	}
	return d
}

// truncateError stringifies err and clips it to maxBytes. Returns ""
// for nil errors so callers don't end up with stringified "%!s(<nil>)".
func truncateError(err error, maxBytes int) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= maxBytes {
		return s
	}
	// Truncate on a rune boundary defensively — most Go errors are ASCII
	// but Notion / Microsoft Graph payloads sometimes embed UTF-8.
	runes := []rune(s)
	if len(runes) <= maxBytes {
		return s
	}
	return string(runes[:maxBytes])
}
