package pipeline

// IndexingInput is the input to an indexing pipeline.
type IndexingInput struct {
	DocumentID string         `json:"document_id"`
	SourceID   string         `json:"source_id"`
	Title      string         `json:"title"`
	Content    string         `json:"content"` // Raw normalized content
	MimeType   string         `json:"mime_type"`
	Path       string         `json:"path"`
	Metadata   map[string]any `json:"metadata"`
}

// IndexingOutput is the final output from an indexing pipeline.
type IndexingOutput struct {
	DocumentID string   `json:"document_id"`
	ChunkIDs   []string `json:"chunk_ids"`
}

// SearchInput is the input to a search pipeline.
type SearchInput struct {
	Query   string        `json:"query"` // Raw user query
	Filters SearchFilters `json:"filters"`
}

// SearchOutput is the final output from a search pipeline.
type SearchOutput struct {
	Results    []PresentedResult  `json:"results"`
	TotalCount int64              `json:"total_count"`
	Facets     map[string][]Facet `json:"facets,omitempty"`
	Timing     ExecutionTiming    `json:"timing"`
}

// PresentedResult is a single search result ready for display.
type PresentedResult struct {
	DocumentID string         `json:"document_id"`
	ChunkID    string         `json:"chunk_id"`
	SourceID   string         `json:"source_id"`
	Title      string         `json:"title"`
	Snippet    string         `json:"snippet"`
	Score      float64        `json:"score"`
	Highlights []Highlight    `json:"highlights,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Highlight represents a highlighted match in content.
type Highlight struct {
	Field  string `json:"field"`
	Text   string `json:"text"`
	Offset int    `json:"offset"`
}

// Facet represents a facet value with count.
type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// ExecutionTiming tracks pipeline execution timing.
type ExecutionTiming struct {
	TotalMs int64            `json:"total_ms"`
	StageMs map[string]int64 `json:"stage_ms"` // Per-stage timing
}

// Chunk represents a piece of content flowing through indexing stages.
type Chunk struct {
	ID          string         `json:"id"`
	DocumentID  string         `json:"document_id"`
	SourceID    string         `json:"source_id"`
	Content     string         `json:"content"`
	Position    int            `json:"position"`     // Chunk index within document
	StartOffset int            `json:"start_offset"` // Character offset from document start
	EndOffset   int            `json:"end_offset"`   // Character offset for chunk end
	Embedding   []float32      `json:"embedding,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Candidate represents a search candidate before final ranking.
type Candidate struct {
	DocumentID string         `json:"document_id"`
	ChunkID    string         `json:"chunk_id"`
	SourceID   string         `json:"source_id"`
	Content    string         `json:"content"`
	Score      float64        `json:"score"`
	Source     string         `json:"source"` // Which retriever found this (e.g., "bm25", "vector")
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ParsedQuery represents a parsed search query with extracted components.
type ParsedQuery struct {
	Original      string        `json:"original"`
	Terms         []string      `json:"terms"`
	Phrases       []string      `json:"phrases,omitempty"`
	Filters       []string      `json:"filters,omitempty"`        // Extracted filter expressions from query text
	SearchFilters SearchFilters `json:"search_filters,omitempty"` // Structured backend filters (source_ids, etc.)
	Intent        string        `json:"intent,omitempty"`         // Detected intent
}
