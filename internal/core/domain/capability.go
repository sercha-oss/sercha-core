package domain

import "time"

// CapabilityType is the open-string identifier for a capability. Built-in
// capabilities ship with constants below; add-ons (or Core consumers wiring
// their own pipeline stages) MAY register additional types via the
// CapabilityRegistry without modifying this file.
type CapabilityType string

const (
	// Indexing capabilities (user controls).
	CapabilityTextIndexing      CapabilityType = "text_indexing"      // Enables BM25 search
	CapabilityEmbeddingIndexing CapabilityType = "embedding_indexing" // Enables vector search

	// Search capabilities (derived from indexing, toggleable).
	CapabilityBM25Search   CapabilityType = "bm25_search"   // Keyword search
	CapabilityVectorSearch CapabilityType = "vector_search" // Semantic search

	// LLM-powered search enhancements (require LLM provider).
	CapabilityQueryExpansion CapabilityType = "query_expansion" // Expands queries with related terms
	CapabilityQueryRewriting CapabilityType = "query_rewriting" // Reformulates queries for better matching
	CapabilitySummarization  CapabilityType = "summarization"   // Generates result snippets
)

// PipelinePhase identifies which phase of the pipeline a capability belongs
// to. The registry uses this to group capabilities for the UI; consumers
// that gate on phase (e.g. a stage factory) may also use it.
type PipelinePhase string

const (
	PipelinePhaseIndexing PipelinePhase = "indexing"
	PipelinePhaseSearch   PipelinePhase = "search"
)

// CapabilityDescriptor is the registered metadata for one capability.
//
// The registry stores descriptors; the capabilities service combines them
// with availability checks and per-team toggle state to produce the final
// list of Capability values served to consumers (UI, pipeline factories).
//
// Descriptors are pure metadata: no behaviour, no backend handles. Backend
// availability is computed by an AvailabilityResolver at request time so a
// running system reflects current state (e.g. an LLM provider added at
// runtime makes LLM-backed capabilities Available without restart).
type CapabilityDescriptor struct {
	// Type is the unique string key. Must match the registry's view.
	Type CapabilityType `json:"type"`

	// DisplayName is the human-readable label rendered in the admin UI.
	DisplayName string `json:"display_name"`

	// Description is one-line guidance text rendered alongside the toggle.
	Description string `json:"description"`

	// Phase indicates whether this is an indexing or search capability.
	Phase PipelinePhase `json:"phase"`

	// BackendID is an optional tag identifying the implementation backend
	// (e.g. "opensearch", "pgvector", "llm"). Used purely for UX
	// (grouping, badges); the capability resolution path does not consult
	// it. Empty when the capability has no specific backend.
	BackendID string `json:"backend_id,omitempty"`

	// DependsOn lists other capability types that must be enabled and
	// available for this capability to function. The resolver cascades
	// disable: if any dependency is not active, this capability is
	// marked disabled.
	DependsOn []CapabilityType `json:"depends_on,omitempty"`

	// Grants lists capabilities that this capability enables. Reverse of
	// DependsOn — purely informational for the UI today.
	Grants []CapabilityType `json:"grants,omitempty"`

	// DefaultEnabled is the toggle's default value when no per-team
	// preference exists. The resolver uses this as the initial enabled
	// state before applying stored preferences.
	DefaultEnabled bool `json:"default_enabled"`
}

// Capability is the resolved runtime state of a single capability for a
// particular team. Built by combining a CapabilityDescriptor with
// availability and persisted preferences.
type Capability struct {
	Type        CapabilityType   `json:"type"`
	DisplayName string           `json:"display_name"`
	Description string           `json:"description"`
	Phase       PipelinePhase    `json:"phase"`
	BackendID   string           `json:"backend_id,omitempty"`
	DependsOn   []CapabilityType `json:"depends_on,omitempty"`
	Grants      []CapabilityType `json:"grants,omitempty"`

	// Available means the runtime can support this capability (backends are
	// configured, services are healthy, etc.). Computed by the resolver.
	Available bool `json:"available"`

	// Enabled means the operator has turned this capability on. Reflects
	// per-team preference, falling back to the descriptor's DefaultEnabled.
	Enabled bool `json:"enabled"`
}

// IsActive reports whether the capability is both available and enabled —
// i.e. ready to use right now.
func (c *Capability) IsActive() bool {
	return c.Available && c.Enabled
}

// IsIndexingCapability reports whether this is an indexing-phase capability.
func (c *Capability) IsIndexingCapability() bool {
	return c.Phase == PipelinePhaseIndexing
}

// IsSearchCapability reports whether this is a search-phase capability.
func (c *Capability) IsSearchCapability() bool {
	return c.Phase == PipelinePhaseSearch
}

// CapabilityPreferences is the persisted per-team toggle state. Stored as
// row-per-toggle in the backing store; presented to consumers as a flat
// map keyed by CapabilityType.
//
// Capabilities absent from the map fall back to the descriptor's
// DefaultEnabled at resolution time. This keeps the storage minimal — only
// explicit operator overrides are persisted.
type CapabilityPreferences struct {
	TeamID    string                  `json:"team_id"`
	Toggles   map[CapabilityType]bool `json:"toggles"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// IsEnabled returns the operator's preference for the given capability,
// defaulting to fallbackDefault when no explicit preference is set. Pass
// the descriptor's DefaultEnabled here so unset toggles inherit the
// capability's intended default rather than always returning false.
func (p *CapabilityPreferences) IsEnabled(t CapabilityType, fallbackDefault bool) bool {
	if p == nil || p.Toggles == nil {
		return fallbackDefault
	}
	v, ok := p.Toggles[t]
	if !ok {
		return fallbackDefault
	}
	return v
}

// DefaultCapabilityPreferences returns an empty preferences struct for a
// team — no toggles set, every capability inherits its descriptor default.
func DefaultCapabilityPreferences(teamID string) *CapabilityPreferences {
	return &CapabilityPreferences{
		TeamID:    teamID,
		Toggles:   make(map[CapabilityType]bool),
		UpdatedAt: time.Now(),
	}
}

// ResolveCapabilities builds the runtime list of resolved Capability values
// from descriptors + availability + persisted preferences. Dependency
// cascades are enforced: a capability whose dependency is not active is
// marked disabled.
//
// The function does not interpret any specific capability type — it walks
// descriptors generically. Adding a new capability requires only
// registering a descriptor (and an availability rule); no code change to
// this function is needed.
func ResolveCapabilities(
	descriptors []CapabilityDescriptor,
	available map[CapabilityType]bool,
	prefs *CapabilityPreferences,
) []*Capability {
	// First pass: build the Capability value for each descriptor with raw
	// available/enabled (no cascade yet).
	caps := make([]*Capability, 0, len(descriptors))
	byType := make(map[CapabilityType]*Capability, len(descriptors))
	for _, d := range descriptors {
		c := &Capability{
			Type:        d.Type,
			DisplayName: d.DisplayName,
			Description: d.Description,
			Phase:       d.Phase,
			BackendID:   d.BackendID,
			DependsOn:   d.DependsOn,
			Grants:      d.Grants,
			Available:   available[d.Type],
			Enabled:     prefs.IsEnabled(d.Type, d.DefaultEnabled),
		}
		caps = append(caps, c)
		byType[d.Type] = c
	}

	// Second pass: cascade-disable. A capability is effectively-disabled
	// when any dependency is not active. Iterate to fixed point so a chain
	// of dependencies (A→B→C) cascades correctly even if descriptors are
	// registered in any order.
	for changed := true; changed; {
		changed = false
		for _, c := range caps {
			if !c.Enabled {
				continue
			}
			for _, dep := range c.DependsOn {
				if depCap, ok := byType[dep]; ok && !depCap.IsActive() {
					c.Enabled = false
					changed = true
					break
				}
			}
		}
	}

	return caps
}
