package pipeline

import (
	"context"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// searchContextKey is an unexported type to prevent key collisions in context.
type searchContextKey struct{}

// SearchContextWithContext stores the SearchContext in ctx, returning a new
// context.  Called by the pipeline executor before invoking each stage's
// Process method so that stages needing caller or filter metadata can
// retrieve it via SearchContextFromContext.
func SearchContextWithContext(ctx context.Context, sctx *SearchContext) context.Context {
	return context.WithValue(ctx, searchContextKey{}, sctx)
}

// SearchContextFromContext retrieves the SearchContext stored by
// SearchContextWithContext.  Returns (nil, false) when no value is present.
func SearchContextFromContext(ctx context.Context) (*SearchContext, bool) {
	v, ok := ctx.Value(searchContextKey{}).(*SearchContext)
	return v, ok
}

// StagePreferences carries the operator's per-team capability toggles into
// the pipeline. Stages and executors read this to decide whether to run.
//
// Toggles is keyed by capability type id (matches domain.CapabilityType
// values). Capabilities absent from the map have no explicit preference
// and the consuming stage should fall back to a sensible default — most
// commonly "enabled". Use IsEnabled to read with an explicit default.
//
// The flat-map shape is the same as domain.CapabilityPreferences. Add-on
// capabilities (e.g. entity_extraction, masking) flow through this map
// without a Core schema change — pipeline stages registered by add-ons
// simply read their own keys.
type StagePreferences struct {
	Toggles map[domain.CapabilityType]bool
}

// IsEnabled returns the persisted preference for the given capability,
// defaulting to fallbackDefault when no explicit preference is set. Pass
// the descriptor's DefaultEnabled here so unset toggles inherit the
// capability's intended default rather than always returning false.
func (p *StagePreferences) IsEnabled(t domain.CapabilityType, fallbackDefault bool) bool {
	if p == nil || p.Toggles == nil {
		return fallbackDefault
	}
	v, ok := p.Toggles[t]
	if !ok {
		return fallbackDefault
	}
	return v
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

	// BoostTerms carries user-supplied keyword boost factors (term →
	// multiplier). Kept on the context for observability and tracing —
	// the consume path retriever stages actually read is
	//   SearchInput.BoostTerms → ParsedQuery.BoostTerms → SearchOptions.BoostTerms
	// (see multi-retriever for the canonical wiring). Stages building
	// their own queries should read q.BoostTerms off the parsed query,
	// not from the context.
	BoostTerms map[string]float64 `json:"boost_terms,omitempty"`

	// Caller identifies the request-source identity for this pipeline run.
	// A nil Caller means the origin is unknown; any consuming stage that
	// branches on caller type MUST treat nil as the most restrictive default
	// (e.g. sensitivity-gated stages should treat nil as non-MCP and skip
	// masking, or equivalently as MCP if fail-closed is safer — follow each
	// stage's documented nil-handling policy).
	Caller *domain.Caller `json:"caller,omitempty"`
}

// SearchFilters contains user-applied search filters.
type SearchFilters struct {
	Sources          []string                 `json:"sources,omitempty"` // Filter by source/connector
	DateRange        *DateRange               `json:"date_range,omitempty"`
	ContentTypes     []string                 `json:"content_types,omitempty"`
	DocumentIDFilter *domain.DocumentIDFilter `json:"document_id_filter,omitempty"` // Three-case filter (see DocumentIDFilter godoc). Nil = no filter.
	Custom           map[string]any           `json:"custom,omitempty"`
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
