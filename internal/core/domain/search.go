package domain

import "time"

// SearchMode determines the search strategy
type SearchMode string

const (
	SearchModeHybrid       SearchMode = "hybrid"   // BM25 + vector (default)
	SearchModeTextOnly     SearchMode = "text"     // BM25 only
	SearchModeSemanticOnly SearchMode = "semantic" // Vector only
)

// SearchOptions configures a search request
type SearchOptions struct {
	Mode        SearchMode         `json:"mode"`
	Limit       int                `json:"limit"`
	Offset      int                `json:"offset"`
	SourceIDs        []string           `json:"source_ids,omitempty"`         // Filter by sources
	DocumentIDFilter *DocumentIDFilter  `json:"document_id_filter,omitempty"` // Three-case filter (see DocumentIDFilter godoc). Nil = no filter.
	Filters          Filters            `json:"filters,omitempty"`
	BoostTerms       map[string]float64 `json:"boost_terms,omitempty"` // term -> boost factor

	// Phrases lists user-quoted phrases extracted by the query parser. Each is
	// run as a match_phrase clause against title^3 and content alongside the
	// main query terms — without this the standard analyser drops the quotes
	// and a "merge sort" search collapses into two unrelated tokens.
	Phrases []string `json:"phrases,omitempty"`

	// PipelineID optionally selects a registered search pipeline by ID.
	// Empty string falls back to the default ("default-search"). Callers
	// can register additional pipelines via PipelineRegistry and route
	// per-request by setting this field.
	PipelineID string `json:"pipeline_id,omitempty"`
}

// Filters provides additional search filters
type Filters struct {
	MimeTypes  []string   `json:"mime_types,omitempty"`
	DateAfter  *time.Time `json:"date_after,omitempty"`
	DateBefore *time.Time `json:"date_before,omitempty"`
}

// DefaultSearchOptions returns sensible defaults
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Mode:   SearchModeHybrid,
		Limit:  20,
		Offset: 0,
	}
}

// SearchResult represents the result of a search query
type SearchResult struct {
	Query      string              `json:"query"`
	Mode       SearchMode          `json:"mode"`
	Results    []*SearchResultItem `json:"results"`
	TotalCount int                 `json:"total_count"`
	Took       time.Duration       `json:"took" swaggertype:"integer" example:"1500000"`
}

// SearchResultItem represents a single search result at document level.
type SearchResultItem struct {
	DocumentID string    `json:"document_id"`
	SourceID   string    `json:"source_id"`
	Title      string    `json:"title"`
	Path       string    `json:"path"`
	MimeType   string    `json:"mime_type"`
	Snippet    string    `json:"snippet"`
	Score      float64   `json:"score"`
	IndexedAt  time.Time `json:"indexed_at"`
}

