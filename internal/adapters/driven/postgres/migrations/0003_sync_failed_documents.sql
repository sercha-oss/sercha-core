-- +goose Up

-- Skip-list / retry-ledger for documents that failed to ingest within a
-- sync run. Today the orchestrator refuses to advance the cursor when
-- any document in a batch errors — a single bad document then blocks
-- every subsequent change indefinitely (every sync re-fetches, re-fails
-- on the same doc, retries the same batch forever).
--
-- This table moves the failure state OUT of the cursor. The orchestrator
-- now always advances the cursor on a fresh delta batch; per-doc errors
-- are recorded here and retried independently with exponential backoff.
-- Once a row reaches the configured terminal attempt count it stays in
-- the table for operator visibility (no auto-cleanup) but is no longer
-- retried until a human resets it.
--
-- Schema is keyed on (source_id, external_id): a successful re-ingest of
-- a doc clears its row; a fresh failure on the same external_id either
-- inserts (first failure) or bumps the existing row (subsequent retry).

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sync_failed_documents (
    source_id           TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    -- The connector's external ID for the document (NOT Sercha's internal
    -- document_id, because failures sometimes happen before we ever store
    -- the document and so have no internal id to key on).
    external_id         TEXT NOT NULL,
    -- Number of times we've attempted this doc (across the lifetime of
    -- the row — re-creating the row resets to 1). Drives the backoff
    -- schedule and the terminal-attempt cutoff.
    attempt_count       INT NOT NULL DEFAULT 1,
    -- The last error message we saw, truncated for storage. Used for
    -- operator triage in admin UIs.
    last_error          TEXT NOT NULL DEFAULT '',
    -- When the most recent attempt ran. Useful for "stuck since X" UIs.
    last_attempted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The earliest time the orchestrator should retry this doc. Computed
    -- as last_attempted_at + min(2^attempt_count * base_delay, max_delay).
    -- Indexed below so the retry pre-pass can scan cheaply per source.
    next_retry_after    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- True once the row has exceeded the configured max attempt count.
    -- Excluded from retry pre-pass; visible in admin UIs as "needs
    -- attention".
    terminal            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_id, external_id)
);
-- +goose StatementEnd

-- Retry pre-pass scans (source_id = ?, terminal = false, next_retry_after <= now())
-- on every sync run. The composite index serves both the equality and
-- range predicates without an extra sort step.
CREATE INDEX IF NOT EXISTS idx_sync_failed_documents_retry
    ON sync_failed_documents(source_id, terminal, next_retry_after);

-- Operator queries in admin UIs filter by source_id then sort by
-- last_attempted_at DESC — keeping the cheap descending sort.
CREATE INDEX IF NOT EXISTS idx_sync_failed_documents_source_recent
    ON sync_failed_documents(source_id, last_attempted_at DESC);


-- +goose Down

DROP INDEX IF EXISTS idx_sync_failed_documents_source_recent;
DROP INDEX IF EXISTS idx_sync_failed_documents_retry;
DROP TABLE IF EXISTS sync_failed_documents;
