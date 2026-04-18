package driven

import "github.com/sercha-oss/sercha-core/internal/core/domain"

// OAuthCredentials holds OAuth client credentials from environment variables
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
}

// AICredentials holds AI provider API keys and base URLs from environment variables
type AICredentials struct {
	APIKey  string
	BaseURL string // Optional custom base URL
}

// Capabilities represents what features are available based on environment configuration
type Capabilities struct {
	// OAuth providers that are configured via environment variables
	OAuthProviders []domain.PlatformType

	// AI providers available for embedding
	EmbeddingProviders []domain.AIProvider

	// AI providers available for LLM
	LLMProviders []domain.AIProvider

	// SearchEngineAvailable indicates if a search engine (e.g. OpenSearch) was initialized
	SearchEngineAvailable bool

	// VectorStoreAvailable indicates if a vector store (e.g. pgvector) was initialized
	VectorStoreAvailable bool

	// Operational boundaries from environment variables
	Limits OperationalLimits
}

// OperationalLimits defines guardrails from environment configuration
type OperationalLimits struct {
	SyncMinInterval   int // Minutes floor (default: 5)
	SyncMaxInterval   int // Minutes ceiling (default: 1440)
	MaxWorkers        int // Worker ceiling (default: 10)
	MaxResultsPerPage int // Results ceiling (default: 100)
}

// ConfigProvider provides access to configuration from environment variables.
// This is a driven port that abstracts environment-based configuration.
// Implementation lives in internal/config/ (infrastructure layer).
type ConfigProvider interface {
	// GetOAuthCredentials returns OAuth client credentials for a platform.
	// Returns nil if the platform is not configured in environment variables.
	GetOAuthCredentials(platform domain.PlatformType) *OAuthCredentials

	// GetAICredentials returns AI provider credentials (API key, base URL).
	// Returns nil if the provider is not configured in environment variables.
	GetAICredentials(provider domain.AIProvider) *AICredentials

	// IsOAuthConfigured returns true if OAuth credentials are available for the platform.
	IsOAuthConfigured(platform domain.PlatformType) bool

	// IsAIConfigured returns true if AI credentials are available for the provider.
	IsAIConfigured(provider domain.AIProvider) bool

	// GetCapabilities returns information about what's available based on env configuration.
	// This is used for the /api/v1/capabilities endpoint.
	GetCapabilities() *Capabilities

	// GetBaseURL returns the application base URL for OAuth callbacks.
	GetBaseURL() string
}
