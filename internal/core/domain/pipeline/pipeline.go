package pipeline

import "time"

// PipelineDefinition is the complete specification of a pipeline.
type PipelineDefinition struct {
	ID          string        `json:"id"`          // Unique identifier (e.g., "default-indexing")
	Name        string        `json:"name"`        // Human-readable name
	Type        PipelineType  `json:"type"`        // "indexing" or "search"
	Stages      []StageConfig `json:"stages"`      // Ordered list of stages
	Version     string        `json:"version"`     // Semantic version
	Description string        `json:"description"` // What this pipeline does
}

// ProducesManifest declares what an indexing pipeline produces.
// Written after indexing completes to track what features exist.
type ProducesManifest struct {
	PipelineID    string               `json:"pipeline_id"`    // Which pipeline produced this
	ConnectorID   string               `json:"connector_id"`   // Which connector/source
	Timestamp     time.Time            `json:"timestamp"`      // When indexing completed
	Capabilities  []ProducedCapability `json:"capabilities"`   // What was stored
	DocumentCount int64                `json:"document_count"` // Stats
	ChunkCount    int64                `json:"chunk_count"`
}

// ProducedCapability describes a capability that was stored during indexing.
type ProducedCapability struct {
	Type       CapabilityType `json:"type"`                 // e.g., "vector_store"
	Store      string         `json:"store"`                // e.g., "pgvector"
	Model      string         `json:"model,omitempty"`      // e.g., "text-embedding-3-small"
	Dimensions int            `json:"dimensions,omitempty"` // For embeddings
	Metadata   map[string]any `json:"metadata,omitempty"`   // Additional info
}

// HasCapability checks if the manifest includes a specific capability type.
func (m *ProducesManifest) HasCapability(capType CapabilityType) bool {
	for _, cap := range m.Capabilities {
		if cap.Type == capType {
			return true
		}
	}
	return false
}

// GetCapability returns the produced capability of a specific type.
func (m *ProducesManifest) GetCapability(capType CapabilityType) *ProducedCapability {
	for i := range m.Capabilities {
		if m.Capabilities[i].Type == capType {
			return &m.Capabilities[i]
		}
	}
	return nil
}

// SearchPipelineRequirements defines what a search pipeline needs to function.
type SearchPipelineRequirements struct {
	PipelineID   string                  `json:"pipeline_id"`  // Which search pipeline
	Capabilities []CapabilityRequirement `json:"capabilities"` // What it needs
}

// SearchPipelineAvailability indicates whether a search pipeline can run.
// Derived from ProducesManifest.
type SearchPipelineAvailability struct {
	PipelineID string   `json:"pipeline_id"` // Search pipeline
	Available  bool     `json:"available"`   // Can it run with current indexed data?
	Missing    []string `json:"missing"`     // What's missing (capability types)
	Degraded   []string `json:"degraded"`    // Optional capabilities not available
}

// SearchPipelineEnablement is admin configuration for active search pipelines.
type SearchPipelineEnablement struct {
	PipelineID string    `json:"pipeline_id"` // Which search pipeline
	Enabled    bool      `json:"enabled"`     // Admin has enabled it
	EnabledAt  time.Time `json:"enabled_at"`  // When it was enabled
	EnabledBy  string    `json:"enabled_by"`  // Who enabled it (admin user ID)
	Priority   int       `json:"priority"`    // For ordering when multiple enabled
}
