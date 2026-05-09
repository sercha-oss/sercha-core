package domain

import "time"

// SyncFailedDoc is the in-memory shape of one row in the
// sync_failed_documents skip-list table. Mirrors the schema 1:1; lives
// in domain so the driven port can reference it without importing
// postgres-specific types.
type SyncFailedDoc struct {
	// SourceID is the parent source — the (source_id, external_id) pair
	// is the primary key.
	SourceID string `json:"source_id"`

	// ExternalID is the connector's identifier for the doc that failed.
	// Not the Sercha-internal document_id: failures often happen before
	// any internal row gets created.
	ExternalID string `json:"external_id"`

	// AttemptCount is the number of times we've tried this doc since the
	// row was created. Drives backoff and the terminal cutoff.
	AttemptCount int `json:"attempt_count"`

	// LastError is the error message from the most recent attempt,
	// truncated by the store layer for storage hygiene.
	LastError string `json:"last_error"`

	// LastAttemptedAt is when the most recent attempt ran. Useful for
	// "stuck since X" presentations in admin UIs.
	LastAttemptedAt time.Time `json:"last_attempted_at"`

	// NextRetryAfter is the earliest time the orchestrator should retry
	// this doc. The retry pre-pass picks rows where
	// next_retry_after <= now() AND terminal = false.
	NextRetryAfter time.Time `json:"next_retry_after"`

	// Terminal is true once the row has exceeded the configured maximum
	// attempt count. Excluded from automatic retry; remains in the table
	// for operator visibility.
	Terminal bool `json:"terminal"`

	// CreatedAt is when the first failure for this (source_id,
	// external_id) was recorded.
	CreatedAt time.Time `json:"created_at"`
}
