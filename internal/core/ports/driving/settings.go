package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// UpdateSettingsRequest represents a request to update settings
// Note: AI configuration is managed via UpdateAISettingsRequest and /settings/ai endpoint
type UpdateSettingsRequest struct {
	DefaultSearchMode     *domain.SearchMode `json:"default_search_mode,omitempty"`
	ResultsPerPage        *int               `json:"results_per_page,omitempty"`
	SyncIntervalMinutes   *int               `json:"sync_interval_minutes,omitempty"`
	SyncEnabled           *bool              `json:"sync_enabled,omitempty"`
	SemanticSearchEnabled *bool              `json:"semantic_search_enabled,omitempty"`
	AutoSuggestEnabled    *bool              `json:"auto_suggest_enabled,omitempty"`
}

// SettingsService manages team-wide settings (admin only)
type SettingsService interface {
	// Get retrieves the current settings
	Get(ctx context.Context) (*domain.Settings, error)

	// Update updates settings (admin only)
	Update(ctx context.Context, updaterID string, req UpdateSettingsRequest) (*domain.Settings, error)

	// GetAISettings retrieves the current AI configuration
	GetAISettings(ctx context.Context) (*domain.AISettings, error)

	// UpdateAISettings updates AI configuration and hot-reloads services
	// Returns the updated settings and whether each service is now available
	UpdateAISettings(ctx context.Context, req UpdateAISettingsRequest) (*AISettingsStatus, error)

	// GetAIStatus returns the current status of AI services
	GetAIStatus(ctx context.Context) (*AISettingsStatus, error)

	// TestConnection tests the AI provider connection
	TestConnection(ctx context.Context) error

	// GetAIProviders returns static metadata about available AI providers
	GetAIProviders(ctx context.Context) (*AIProvidersResponse, error)
}

// UpdateAISettingsRequest represents a request to update AI settings
type UpdateAISettingsRequest struct {
	Embedding *EmbeddingSettingsInput `json:"embedding,omitempty"`
	LLM       *LLMSettingsInput       `json:"llm,omitempty"`
}

// EmbeddingSettingsInput is the input for embedding configuration.
// API keys and base URLs come from environment variables, not request input.
type EmbeddingSettingsInput struct {
	Provider domain.AIProvider `json:"provider"`
	Model    string            `json:"model"`
}

// LLMSettingsInput is the input for LLM configuration.
// API keys and base URLs come from environment variables, not request input.
type LLMSettingsInput struct {
	Provider domain.AIProvider `json:"provider"`
	Model    string            `json:"model"`
}

// AISettingsStatus represents the status of AI services
type AISettingsStatus struct {
	Embedding           AIServiceStatus   `json:"embedding"`
	LLM                 AIServiceStatus   `json:"llm"`
	EffectiveSearchMode domain.SearchMode `json:"effective_search_mode"`
}

// AIServiceStatus represents the status of a single AI service
type AIServiceStatus struct {
	Available    bool              `json:"available"`
	Provider     domain.AIProvider `json:"provider,omitempty"`
	Model        string            `json:"model,omitempty"`
	EmbeddingDim int               `json:"embedding_dim,omitempty"` // Only for embedding service
}

// AIProvidersResponse represents the list of available AI providers with their metadata
type AIProvidersResponse struct {
	Embedding []domain.AIProviderInfo `json:"embedding"`
	LLM       []domain.AIProviderInfo `json:"llm"`
}
