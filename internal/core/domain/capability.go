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

// DefaultCapabilityPreferences returns default preferences for a team.
func DefaultCapabilityPreferences(teamID string) *CapabilityPreferences {
	return &CapabilityPreferences{
		TeamID:                   teamID,
		TextIndexingEnabled:      true,  // BM25 enabled by default
		EmbeddingIndexingEnabled: false, // Vectors disabled by default (requires AI setup)
		BM25SearchEnabled:        true,  // BM25 search enabled by default
		VectorSearchEnabled:      true,  // Vector search enabled by default (when available)
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
