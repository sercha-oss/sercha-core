package domain

import "time"

// SyncEvent represents a logged sync operation for audit and analytics
// This is an entity (has identity) used to track sync history and performance
type SyncEvent struct {
	// ID is the unique identifier for this sync event log entry
	ID string `json:"id"`

	// TeamID is the team that owns the source being synced
	TeamID string `json:"team_id"`

	// SourceID is the source that was synced
	SourceID string `json:"source_id"`

	// SourceName is the display name of the source at sync time
	SourceName string `json:"source_name"`

	// ProviderType identifies the connector type (github, notion, etc.)
	ProviderType ProviderType `json:"provider_type"`

	// Status indicates if the sync completed successfully or failed
	Status SyncStatus `json:"status"`

	// DocumentsAdded is the number of new documents discovered
	DocumentsAdded int `json:"documents_added"`

	// DocumentsUpdated is the number of existing documents modified
	DocumentsUpdated int `json:"documents_updated"`

	// DocumentsDeleted is the number of documents removed
	DocumentsDeleted int `json:"documents_deleted"`

	// ChunksIndexed is the total number of chunks created/updated in the vector store
	ChunksIndexed int `json:"chunks_indexed"`

	// ErrorCount is the number of errors encountered during sync
	ErrorCount int `json:"error_count"`

	// ErrorMessage contains the primary error message if Status is failed
	ErrorMessage string `json:"error_message,omitempty"`

	// DurationSeconds is how long the sync operation took to execute
	DurationSeconds float64 `json:"duration_seconds"`

	// CreatedAt is when the sync event was logged
	CreatedAt time.Time `json:"created_at"`
}

// NewSyncEvent creates a new sync event log entry for a completed sync
func NewSyncEvent(
	teamID string,
	sourceID string,
	sourceName string,
	providerType ProviderType,
	status SyncStatus,
	stats SyncStats,
	durationSeconds float64,
) *SyncEvent {
	return &SyncEvent{
		ID:               GenerateID(),
		TeamID:           teamID,
		SourceID:         sourceID,
		SourceName:       sourceName,
		ProviderType:     providerType,
		Status:           status,
		DocumentsAdded:   stats.DocumentsAdded,
		DocumentsUpdated: stats.DocumentsUpdated,
		DocumentsDeleted: stats.DocumentsDeleted,
		ChunksIndexed:    stats.ChunksIndexed,
		ErrorCount:       stats.Errors,
		DurationSeconds:  durationSeconds,
		CreatedAt:        time.Now(),
	}
}

// WithError sets the error message for a failed sync event
func (se *SyncEvent) WithError(errMsg string) *SyncEvent {
	se.ErrorMessage = errMsg
	return se
}

// IsSuccessful returns true if the sync completed without failure
func (se *SyncEvent) IsSuccessful() bool {
	return se.Status == SyncStatusCompleted
}

// IsFailed returns true if the sync failed
func (se *SyncEvent) IsFailed() bool {
	return se.Status == SyncStatusFailed
}

// TotalDocuments returns the total number of documents affected by the sync
func (se *SyncEvent) TotalDocuments() int {
	return se.DocumentsAdded + se.DocumentsUpdated + se.DocumentsDeleted
}

// GetDuration returns the duration as a time.Duration
func (se *SyncEvent) GetDuration() time.Duration {
	return time.Duration(se.DurationSeconds * float64(time.Second))
}
