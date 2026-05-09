package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// CapabilitiesService exposes capability descriptors, runtime availability,
// and per-team toggles to driving adapters (HTTP API, MCP, etc.).
//
// The service is registry-driven: it iterates the registered descriptors
// and reports state for each. New capabilities (Core built-ins or add-on
// registered) appear automatically without service-side code changes.
type CapabilitiesService interface {
	// GetCapabilities returns the full capability snapshot for a team —
	// OAuth providers, AI providers, registered capability descriptors
	// + their resolved status, and operational limits.
	GetCapabilities(ctx context.Context, teamID string) (*CapabilitiesResponse, error)

	// GetCapabilityPreferences returns the persisted toggle state for the
	// team. Capabilities absent from the result have no explicit
	// preference and fall back to descriptor defaults.
	GetCapabilityPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error)

	// UpdateCapabilityPreferences applies a partial toggle update for the
	// team. Toggles not present in the request are left unchanged.
	// Returns the resulting preferences (post-update).
	UpdateCapabilityPreferences(ctx context.Context, teamID string, req UpdateCapabilityPreferencesRequest) (*domain.CapabilityPreferences, error)
}

// UpdateCapabilityPreferencesRequest is a partial-update DTO. Each entry
// in Toggles becomes one upsert against (team_id, capability_type).
// Capabilities not in the map are untouched in storage.
//
// Toggle keys are domain.CapabilityType values; the map deliberately uses
// the same type as the rest of the domain so callers cannot accidentally
// pass arbitrary strings without going through the type.
type UpdateCapabilityPreferencesRequest struct {
	Toggles map[domain.CapabilityType]bool `json:"toggles"`
}

// CapabilitiesResponse is the full capability snapshot for a team.
type CapabilitiesResponse struct {
	// OAuthProviders lists OAuth platforms configured via environment.
	OAuthProviders []domain.PlatformType `json:"oauth_providers"`

	// AIProviders lists AI providers available for embedding and LLM.
	AIProviders AIProvidersCapability `json:"ai_providers"`

	// Descriptors lists every registered capability's metadata. Drives
	// dynamic UI rendering — the admin panel iterates this list to know
	// what toggles to show.
	Descriptors []domain.CapabilityDescriptor `json:"descriptors"`

	// Features maps each registered capability's type to its resolved
	// runtime status (available + enabled + active). Stable counterpart
	// to Descriptors: callers cross-reference by Type.
	Features map[domain.CapabilityType]CapabilityStatus `json:"features"`

	// Limits defines operational boundaries from environment configuration.
	Limits LimitsCapability `json:"limits"`
}

// AIProvidersCapability lists available AI providers.
type AIProvidersCapability struct {
	Embedding []domain.AIProvider `json:"embedding"`
	LLM       []domain.AIProvider `json:"llm"`
}

// CapabilityStatus is the resolved runtime state of one capability.
type CapabilityStatus struct {
	// Available reports whether the runtime can support this capability
	// right now (backends configured, services healthy).
	Available bool `json:"available"`

	// Enabled reports the operator's preference (or the descriptor default
	// when no explicit preference is stored).
	Enabled bool `json:"enabled"`

	// Active is Available AND Enabled.
	Active bool `json:"active"`
}

// LimitsCapability defines operational boundaries.
type LimitsCapability struct {
	SyncMinInterval   int `json:"sync_min_interval"`
	SyncMaxInterval   int `json:"sync_max_interval"`
	MaxWorkers        int `json:"max_workers"`
	MaxResultsPerPage int `json:"max_results_per_page"`
}
