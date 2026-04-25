package domain

import "time"

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

// Change represents a document change from a connector
type Change struct {
	Type       ChangeType `json:"type"`
	Document   *Document  `json:"document,omitempty"`   // For added/modified
	Content    string     `json:"content,omitempty"`    // Raw content for added/modified
	DeletedID  string     `json:"deleted_id,omitempty"` // For deleted
	ExternalID string     `json:"external_id"`          // ID from source system
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
