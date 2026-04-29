package domain

import (
	"context"
	"time"
)

// SyncStatus represents the current state of a sync operation
type SyncStatus string

const (
	SyncStatusIdle      SyncStatus = "idle"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

// SyncState tracks the sync state for a source
type SyncState struct {
	SourceID    string     `json:"source_id"`
	Status      SyncStatus `json:"status"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`
	NextSyncAt  *time.Time `json:"next_sync_at,omitempty"`
	Cursor      string     `json:"cursor,omitempty"` // Pagination cursor for incremental sync
	Stats       SyncStats  `json:"stats"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// SyncStats holds statistics for a sync operation
type SyncStats struct {
	DocumentsAdded   int `json:"documents_added"`
	DocumentsUpdated int `json:"documents_updated"`
	DocumentsDeleted int `json:"documents_deleted"`
	ChunksIndexed    int `json:"chunks_indexed"`
	Errors           int `json:"errors"`
}

// ChangeType indicates what happened to a document
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeDeleted  ChangeType = "deleted"
)

// Change represents a document change from a connector.
//
// Content resolution: when LoadContent is non-nil, the orchestrator invokes
// it lazily during processing and uses the returned bytes as the document
// content; otherwise the eager Content field is used. Connectors with
// non-trivial body fetches SHOULD populate LoadContent rather than
// downloading inline during FetchChanges, so listing returns quickly and
// downloads parallelise with the per-document worker pool.
//
// LoadContent is intentionally not serialised — the closure captures
// connector-scoped state (HTTP client, auth) that is not portable across
// process boundaries.
type Change struct {
	Type        ChangeType                                  `json:"type"`
	Document    *Document                                   `json:"document,omitempty"`   // For added/modified
	Content     string                                      `json:"content,omitempty"`    // Eager content; ignored when LoadContent is set
	LoadContent func(ctx context.Context) (string, error)   `json:"-"`                    // Optional. When set, invoked once during processing to fetch content lazily.
	DeletedID   string                                      `json:"deleted_id,omitempty"` // For deleted
	ExternalID  string                                      `json:"external_id"`          // ID from source system
}

// SyncResult represents the outcome of a sync operation
type SyncResult struct {
	SourceID string    `json:"source_id"`
	Success  bool      `json:"success"`
	Skipped  bool      `json:"skipped,omitempty"` // true when another sync was already in progress for this source
	Stats    SyncStats `json:"stats"`
	Error    string    `json:"error,omitempty"`
	Duration float64   `json:"duration_seconds"`
	Cursor   string    `json:"cursor,omitempty"`
}
