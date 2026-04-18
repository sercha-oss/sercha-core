package pipeline

import "time"

// StagePreferences controls which stages are enabled based on user settings.
type StagePreferences struct {
	TextIndexingEnabled      bool
	EmbeddingIndexingEnabled bool
	BM25SearchEnabled        bool
	VectorSearchEnabled      bool
}

// IndexingContext is the runtime context for indexing pipeline execution.
type IndexingContext struct {
	PipelineID   string            `json:"pipeline_id"`
	ConnectorID  string            `json:"connector_id"`
	SourceID     string            `json:"source_id"`
	RunID        string            `json:"run_id"` // Unique execution ID
	Capabilities *CapabilitySet    `json:"-"`      // Available capabilities for this run
	Preferences  *StagePreferences `json:"-"`      // User capability preferences
	Metadata     map[string]any    `json:"metadata"`
}

// SearchContext is the runtime context for search pipeline execution.
type SearchContext struct {
	PipelineID   string            `json:"pipeline_id"`
	UserID       string            `json:"user_id"`    // Who is searching
	SessionID    string            `json:"session_id"` // For tracking
	RunID        string            `json:"run_id"`     // Unique execution ID
	Capabilities *CapabilitySet    `json:"-"`          // Available capabilities
	Preferences  *StagePreferences `json:"-"`          // User capability preferences
	Filters      SearchFilters     `json:"filters"`
	Pagination   PaginationConfig  `json:"pagination"`
}

// SearchFilters contains user-applied search filters.
type SearchFilters struct {
	Sources      []string       `json:"sources,omitempty"` // Filter by source/connector
	DateRange    *DateRange     `json:"date_range,omitempty"`
	ContentTypes []string       `json:"content_types,omitempty"`
	Custom       map[string]any `json:"custom,omitempty"`
}

// DateRange specifies a time range for filtering.
type DateRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

// PaginationConfig specifies pagination parameters.
type PaginationConfig struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// DefaultPagination returns default pagination settings.
func DefaultPagination() PaginationConfig {
	return PaginationConfig{
		Offset: 0,
		Limit:  20,
	}
}
