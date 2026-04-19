package domain

import "time"

// CapabilityType represents a specific capability in the pipeline
type CapabilityType string

const (
	// Indexing capabilities (user controls)
	CapabilityTextIndexing      CapabilityType = "text_indexing"      // Enables BM25 search
	CapabilityEmbeddingIndexing CapabilityType = "embedding_indexing" // Enables vector search

	// Search capabilities (derived from indexing, toggleable)
	CapabilityBM25Search   CapabilityType = "bm25_search"   // Keyword search
	CapabilityVectorSearch CapabilityType = "vector_search" // Semantic search

	// LLM-powered search enhancements (requires LLM provider)
	CapabilityQueryExpansion CapabilityType = "query_expansion" // Expands queries with related terms
	CapabilityQueryRewriting CapabilityType = "query_rewriting" // Reformulates queries for better matching
	CapabilitySummarization  CapabilityType = "summarization"   // Generates result snippets
)

// PipelinePhase represents which phase of the pipeline a capability belongs to
type PipelinePhase string

const (
	PipelinePhaseIndexing PipelinePhase = "indexing" // Document ingestion and processing
	PipelinePhaseSearch   PipelinePhase = "search"   // Query execution
)

// Capability represents a specific capability in the search pipeline.
// It defines what the system can do (indexing or search) and its current state.
type Capability struct {
	// ID is a unique identifier for this capability instance
	ID string `json:"id"`

	// Type identifies which capability this is
	Type CapabilityType `json:"type"`

	// Phase indicates whether this is an indexing or search capability
	Phase PipelinePhase `json:"phase"`

	// BackendID identifies which backend provides this capability (e.g., "opensearch", "pgvector")
	// Empty for capabilities that don't require a specific backend
	BackendID string `json:"backend_id,omitempty"`

	// Available indicates if this capability is available based on environment and health checks
	// False means the backend is not configured or not healthy
	Available bool `json:"available"`

	// Enabled indicates if the user has enabled this capability
	// User can disable available capabilities
	Enabled bool `json:"enabled"`

	// Grants lists capabilities that this capability enables
	// Example: text_indexing grants bm25_search
	Grants []CapabilityType `json:"grants,omitempty"`

	// DependsOn lists capabilities that must be enabled for this to work
	// Example: bm25_search depends on text_indexing
	DependsOn []CapabilityType `json:"depends_on,omitempty"`
}

// CapabilityPreferences stores per-team capability preferences.
// This is the persisted state that users control through the admin UI.
type CapabilityPreferences struct {
	// TeamID is the team these preferences belong to
	TeamID string `json:"team_id"`

	// Indexing preferences (explicitly controlled by user)
	TextIndexingEnabled      bool `json:"text_indexing_enabled"`      // Enable BM25 indexing
	EmbeddingIndexingEnabled bool `json:"embedding_indexing_enabled"` // Enable vector indexing

	// Search preferences (can toggle down, but requires indexing to be enabled)
	BM25SearchEnabled   bool `json:"bm25_search_enabled"`   // Enable BM25 search (if text indexing enabled)
	VectorSearchEnabled bool `json:"vector_search_enabled"` // Enable vector search (if embedding indexing enabled)

	// LLM-powered search enhancements (requires LLM provider)
	QueryExpansionEnabled bool `json:"query_expansion_enabled"` // Enable query expansion
	QueryRewritingEnabled bool `json:"query_rewriting_enabled"` // Enable query rewriting
	SummarizationEnabled  bool `json:"summarization_enabled"`   // Enable result summarization

	// UpdatedAt tracks when preferences were last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// IsActive returns true if the capability is both available and enabled.
// This is the primary indicator that a capability can be used.
func (c *Capability) IsActive() bool {
	return c.Available && c.Enabled
}

// CanBeEnabled returns true if the capability is available (can be enabled by user).
func (c *Capability) CanBeEnabled() bool {
	return c.Available
}

// IsIndexingCapability returns true if this is an indexing phase capability.
func (c *Capability) IsIndexingCapability() bool {
	return c.Phase == PipelinePhaseIndexing
}

// IsSearchCapability returns true if this is a search phase capability.
func (c *Capability) IsSearchCapability() bool {
	return c.Phase == PipelinePhaseSearch
}

// HasBackend returns true if this capability requires a specific backend.
func (c *Capability) HasBackend() bool {
	return c.BackendID != ""
}

// NewTextIndexingCapability creates a text indexing capability with defaults.
func NewTextIndexingCapability(backendID string, available bool) *Capability {
	return &Capability{
		ID:        "text_indexing",
		Type:      CapabilityTextIndexing,
		Phase:     PipelinePhaseIndexing,
		BackendID: backendID,
		Available: available,
		Enabled:   true, // Default enabled
		Grants:    []CapabilityType{CapabilityBM25Search},
	}
}

// NewEmbeddingIndexingCapability creates an embedding indexing capability with defaults.
func NewEmbeddingIndexingCapability(backendID string, available bool) *Capability {
	return &Capability{
		ID:        "embedding_indexing",
		Type:      CapabilityEmbeddingIndexing,
		Phase:     PipelinePhaseIndexing,
		BackendID: backendID,
		Available: available,
		Enabled:   false, // Default disabled (requires AI provider)
		Grants:    []CapabilityType{CapabilityVectorSearch},
	}
}

// NewBM25SearchCapability creates a BM25 search capability with defaults.
func NewBM25SearchCapability(backendID string, available bool) *Capability {
	return &Capability{
		ID:        "bm25_search",
		Type:      CapabilityBM25Search,
		Phase:     PipelinePhaseSearch,
		BackendID: backendID,
		Available: available,
		Enabled:   true, // Default enabled
		DependsOn: []CapabilityType{CapabilityTextIndexing},
	}
}

// NewVectorSearchCapability creates a vector search capability with defaults.
func NewVectorSearchCapability(backendID string, available bool) *Capability {
	return &Capability{
		ID:        "vector_search",
		Type:      CapabilityVectorSearch,
		Phase:     PipelinePhaseSearch,
		BackendID: backendID,
		Available: available,
		Enabled:   true, // Default enabled (but depends on embedding indexing)
		DependsOn: []CapabilityType{CapabilityEmbeddingIndexing},
	}
}

// NewQueryExpansionCapability creates a query expansion capability with defaults.
func NewQueryExpansionCapability(available bool) *Capability {
	return &Capability{
		ID:        "query_expansion",
		Type:      CapabilityQueryExpansion,
		Phase:     PipelinePhaseSearch,
		BackendID: "llm",
		Available: available,
		Enabled:   true, // Default enabled when LLM is available
	}
}

// NewQueryRewritingCapability creates a query rewriting capability with defaults.
func NewQueryRewritingCapability(available bool) *Capability {
	return &Capability{
		ID:        "query_rewriting",
		Type:      CapabilityQueryRewriting,
		Phase:     PipelinePhaseSearch,
		BackendID: "llm",
		Available: available,
		Enabled:   true, // Default enabled when LLM is available
	}
}

// NewSummarizationCapability creates a summarization capability with defaults.
func NewSummarizationCapability(available bool) *Capability {
	return &Capability{
		ID:        "summarization",
		Type:      CapabilitySummarization,
		Phase:     PipelinePhaseSearch,
		BackendID: "llm",
		Available: available,
		Enabled:   true, // Default enabled when LLM is available
	}
}

// DefaultCapabilityPreferences returns default preferences for a team.
func DefaultCapabilityPreferences(teamID string) *CapabilityPreferences {
	return &CapabilityPreferences{
		TeamID:                   teamID,
		TextIndexingEnabled:      true,  // BM25 enabled by default
		EmbeddingIndexingEnabled: false, // Vectors disabled by default (requires AI setup)
		BM25SearchEnabled:        true,  // BM25 search enabled by default
		VectorSearchEnabled:      true,  // Vector search enabled by default (when available)
		QueryExpansionEnabled:    true,  // LLM features enabled by default (when available)
		QueryRewritingEnabled:    true,
		SummarizationEnabled:     true,
		UpdatedAt:                time.Now(),
	}
}

// HasTextIndexing returns true if text indexing is enabled.
func (p *CapabilityPreferences) HasTextIndexing() bool {
	return p.TextIndexingEnabled
}

// HasEmbeddingIndexing returns true if embedding indexing is enabled.
func (p *CapabilityPreferences) HasEmbeddingIndexing() bool {
	return p.EmbeddingIndexingEnabled
}

// CanUseBM25Search returns true if BM25 search can be used.
// Requires both text indexing and BM25 search to be enabled.
func (p *CapabilityPreferences) CanUseBM25Search() bool {
	return p.TextIndexingEnabled && p.BM25SearchEnabled
}

// CanUseVectorSearch returns true if vector search can be used.
// Requires both embedding indexing and vector search to be enabled.
func (p *CapabilityPreferences) CanUseVectorSearch() bool {
	return p.EmbeddingIndexingEnabled && p.VectorSearchEnabled
}

// EnableTextIndexing enables text indexing and BM25 search.
func (p *CapabilityPreferences) EnableTextIndexing() {
	p.TextIndexingEnabled = true
	p.BM25SearchEnabled = true
	p.UpdatedAt = time.Now()
}

// DisableTextIndexing disables text indexing and BM25 search.
func (p *CapabilityPreferences) DisableTextIndexing() {
	p.TextIndexingEnabled = false
	p.BM25SearchEnabled = false
	p.UpdatedAt = time.Now()
}

// EnableEmbeddingIndexing enables embedding indexing and vector search.
func (p *CapabilityPreferences) EnableEmbeddingIndexing() {
	p.EmbeddingIndexingEnabled = true
	p.VectorSearchEnabled = true
	p.UpdatedAt = time.Now()
}

// DisableEmbeddingIndexing disables embedding indexing and vector search.
func (p *CapabilityPreferences) DisableEmbeddingIndexing() {
	p.EmbeddingIndexingEnabled = false
	p.VectorSearchEnabled = false
	p.UpdatedAt = time.Now()
}

// ResolveCapabilities builds runtime capability state by combining backend availability
// with user preferences. The available map uses domain CapabilityType keys.
//
// Availability mapping:
// - CapabilityTextIndexing: whether OpenSearch is available
// - CapabilityEmbeddingIndexing: whether embeddings + vector store are available
// - CapabilityBM25Search: same as text_indexing (derived)
// - CapabilityVectorSearch: same as embedding_indexing (derived)
// - CapabilityQueryExpansion: whether LLM is available
// - CapabilityQueryRewriting: whether LLM is available
// - CapabilitySummarization: whether LLM is available
//
// Dependency rules are enforced: if a dependency is not enabled, the dependent
// capability is also not enabled (e.g., bm25_search requires text_indexing).
func ResolveCapabilities(prefs *CapabilityPreferences, available map[CapabilityType]bool) []*Capability {
	// Create capabilities using factory functions, passing availability from the map
	textIndexing := NewTextIndexingCapability("opensearch", available[CapabilityTextIndexing])
	embeddingIndexing := NewEmbeddingIndexingCapability("pgvector", available[CapabilityEmbeddingIndexing])
	bm25Search := NewBM25SearchCapability("opensearch", available[CapabilityBM25Search])
	vectorSearch := NewVectorSearchCapability("pgvector", available[CapabilityVectorSearch])

	// LLM-powered capabilities
	llmAvailable := available[CapabilityQueryExpansion] // All LLM caps share same availability
	queryExpansion := NewQueryExpansionCapability(llmAvailable)
	queryRewriting := NewQueryRewritingCapability(llmAvailable)
	summarization := NewSummarizationCapability(llmAvailable)

	// Apply user preferences if provided
	if prefs != nil {
		textIndexing.Enabled = prefs.TextIndexingEnabled
		embeddingIndexing.Enabled = prefs.EmbeddingIndexingEnabled
		bm25Search.Enabled = prefs.BM25SearchEnabled
		vectorSearch.Enabled = prefs.VectorSearchEnabled
		queryExpansion.Enabled = prefs.QueryExpansionEnabled
		queryRewriting.Enabled = prefs.QueryRewritingEnabled
		summarization.Enabled = prefs.SummarizationEnabled
	}

	// Enforce dependency rules:
	// bm25_search depends on text_indexing
	if !textIndexing.Enabled || !textIndexing.Available {
		bm25Search.Enabled = false
	}

	// vector_search depends on embedding_indexing
	if !embeddingIndexing.Enabled || !embeddingIndexing.Available {
		vectorSearch.Enabled = false
	}

	// LLM features don't have dependencies, but if LLM is not available, they can't be enabled
	if !llmAvailable {
		queryExpansion.Enabled = false
		queryRewriting.Enabled = false
		summarization.Enabled = false
	}

	return []*Capability{
		textIndexing, embeddingIndexing, bm25Search, vectorSearch,
		queryExpansion, queryRewriting, summarization,
	}
}
