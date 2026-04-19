package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// CapabilitiesService provides information about what features are available
// based on environment configuration (env vars) and per-team preferences.
type CapabilitiesService interface {
	// GetCapabilities returns information about available features.
	// teamID is used to resolve per-team preferences; if empty, defaults are used.
	GetCapabilities(ctx context.Context, teamID string) (*CapabilitiesResponse, error)

	// GetCapabilityPreferences retrieves capability preferences for a team.
	GetCapabilityPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error)

	// UpdateCapabilityPreferences updates capability preferences for a team.
	// Uses partial update semantics - only non-nil fields are applied.
	UpdateCapabilityPreferences(ctx context.Context, teamID string, req UpdateCapabilityPreferencesRequest) (*domain.CapabilityPreferences, error)
}

// UpdateCapabilityPreferencesRequest represents a request to update capability preferences.
// All fields are optional pointers to support partial updates.
type UpdateCapabilityPreferencesRequest struct {
	// TextIndexingEnabled controls BM25 text indexing
	TextIndexingEnabled *bool `json:"text_indexing_enabled,omitempty"`

	// EmbeddingIndexingEnabled controls vector/embedding indexing
	EmbeddingIndexingEnabled *bool `json:"embedding_indexing_enabled,omitempty"`

	// BM25SearchEnabled controls BM25 search (requires text indexing)
	BM25SearchEnabled *bool `json:"bm25_search_enabled,omitempty"`

	// VectorSearchEnabled controls vector search (requires embedding indexing)
	VectorSearchEnabled *bool `json:"vector_search_enabled,omitempty"`

	// QueryExpansionEnabled controls LLM-powered query expansion
	QueryExpansionEnabled *bool `json:"query_expansion_enabled,omitempty"`

	// QueryRewritingEnabled controls LLM-powered query rewriting
	QueryRewritingEnabled *bool `json:"query_rewriting_enabled,omitempty"`

	// SummarizationEnabled controls LLM-powered result summarization
	SummarizationEnabled *bool `json:"summarization_enabled,omitempty"`
}

// CapabilitiesResponse represents the capabilities available to the application.
// @Description Information about what features are enabled via environment configuration
type CapabilitiesResponse struct {
	// OAuthProviders lists OAuth platforms configured via environment variables
	OAuthProviders []domain.PlatformType `json:"oauth_providers"`

	// AIProviders lists AI providers available for embedding and LLM
	AIProviders AIProvidersCapability `json:"ai_providers"`

	// Features lists feature flags
	Features FeaturesCapability `json:"features"`

	// Limits defines operational boundaries from environment configuration
	Limits LimitsCapability `json:"limits"`
}

// AIProvidersCapability lists available AI providers
type AIProvidersCapability struct {
	// Embedding lists providers available for embedding service
	Embedding []domain.AIProvider `json:"embedding"`

	// LLM lists providers available for LLM service
	LLM []domain.AIProvider `json:"llm"`
}

// CapabilityStatus represents the state of a single capability.
type CapabilityStatus struct {
	// Available indicates if the backend is configured and healthy
	Available bool `json:"available"`

	// Enabled indicates if the user has enabled this capability
	Enabled bool `json:"enabled"`

	// Active indicates both available AND enabled
	Active bool `json:"active"`
}

// FeaturesCapability lists backend availability for each capability type.
// These map directly to domain.CapabilityType and combine backend availability
// with user preferences to show the full capability state.
type FeaturesCapability struct {
	// TextIndexing indicates BM25 text indexing capability state (requires search engine e.g. OpenSearch)
	TextIndexing CapabilityStatus `json:"text_indexing"`

	// EmbeddingIndexing indicates embedding indexing capability state (requires embedding service + vector store)
	EmbeddingIndexing CapabilityStatus `json:"embedding_indexing"`

	// BM25Search indicates BM25 keyword search capability state (requires search engine)
	BM25Search CapabilityStatus `json:"bm25_search"`

	// VectorSearch indicates vector similarity search capability state (requires embedding service + vector store)
	VectorSearch CapabilityStatus `json:"vector_search"`

	// QueryExpansion indicates LLM-powered query expansion capability state (requires LLM provider)
	QueryExpansion CapabilityStatus `json:"query_expansion"`

	// QueryRewriting indicates LLM-powered query rewriting capability state (requires LLM provider)
	QueryRewriting CapabilityStatus `json:"query_rewriting"`

	// Summarization indicates LLM-powered result summarization capability state (requires LLM provider)
	Summarization CapabilityStatus `json:"summarization"`
}

// LimitsCapability defines operational boundaries
type LimitsCapability struct {
	// SyncMinInterval is the minimum sync interval in minutes
	SyncMinInterval int `json:"sync_min_interval"`

	// SyncMaxInterval is the maximum sync interval in minutes
	SyncMaxInterval int `json:"sync_max_interval"`

	// MaxWorkers is the maximum number of sync workers
	MaxWorkers int `json:"max_workers"`

	// MaxResultsPerPage is the maximum results per page
	MaxResultsPerPage int `json:"max_results_per_page"`
}
