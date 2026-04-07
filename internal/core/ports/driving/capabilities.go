package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// CapabilitiesService provides information about what features are available
// based on environment configuration (env vars).
type CapabilitiesService interface {
	// GetCapabilities returns information about available features.
	GetCapabilities(ctx context.Context) (*CapabilitiesResponse, error)
}

// CapabilitiesResponse represents the capabilities available to the application.
// @Description Information about what features are enabled via environment configuration
type CapabilitiesResponse struct {
	// OAuthProviders lists OAuth providers configured via environment variables
	OAuthProviders []domain.ProviderType `json:"oauth_providers"`

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

// FeaturesCapability lists available features
type FeaturesCapability struct {
	// SemanticSearch indicates if semantic search is available (requires embedding service)
	SemanticSearch bool `json:"semantic_search"`

	// VectorIndexing indicates if vector indexing is available (requires vector store + embeddings)
	VectorIndexing bool `json:"vector_indexing"`
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
