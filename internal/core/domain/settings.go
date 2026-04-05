package domain

import "time"

// AIProvider identifies the AI/embedding provider
type AIProvider string

const (
	AIProviderOpenAI    AIProvider = "openai"
	AIProviderAnthropic AIProvider = "anthropic"
	AIProviderOllama    AIProvider = "ollama"
	AIProviderCohere    AIProvider = "cohere"
	AIProviderVoyage    AIProvider = "voyage"
)

// Settings holds team-wide configuration
// Note: AI configuration (provider, model, API keys) is managed via AISettings
// and the /settings/ai endpoint, not here.
type Settings struct {
	TeamID string `json:"team_id"`

	// Search Defaults
	DefaultSearchMode SearchMode `json:"default_search_mode"`
	ResultsPerPage    int        `json:"results_per_page"`
	MaxResultsPerPage int        `json:"max_results_per_page"`

	// Sync Configuration
	SyncIntervalMinutes int  `json:"sync_interval_minutes"`
	SyncEnabled         bool `json:"sync_enabled"`

	// Feature Flags
	SemanticSearchEnabled bool `json:"semantic_search_enabled"`
	AutoSuggestEnabled    bool `json:"auto_suggest_enabled"`

	// Metadata
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"` // User ID
}

// DefaultSettings returns sensible defaults for a new team
func DefaultSettings(teamID string) *Settings {
	return &Settings{
		TeamID:                teamID,
		DefaultSearchMode:     SearchModeHybrid,
		ResultsPerPage:        20,
		MaxResultsPerPage:     100,
		SyncIntervalMinutes:   60,
		SyncEnabled:           true,
		SemanticSearchEnabled: true,
		AutoSuggestEnabled:    true,
		UpdatedAt:             time.Now(),
	}
}


// EmbeddingConfig holds embedding model configuration
type EmbeddingConfig struct {
	Provider   AIProvider `json:"provider"`
	Model      string     `json:"model"`
	Dimensions int        `json:"dimensions"`
	BatchSize  int        `json:"batch_size"`
}

// DefaultEmbeddingConfig returns default embedding configuration
func DefaultEmbeddingConfig() *EmbeddingConfig {
	return &EmbeddingConfig{
		Provider:   AIProviderOpenAI,
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		BatchSize:  100,
	}
}

// AISettings holds AI service configuration (embedding and LLM)
// This can be updated at runtime via API
type AISettings struct {
	TeamID    string            `json:"team_id"`
	Embedding EmbeddingSettings `json:"embedding"`
	LLM       LLMSettings       `json:"llm"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// EmbeddingSettings configures the embedding service
// API keys and base URLs come from environment variables, not stored here
type EmbeddingSettings struct {
	Provider AIProvider `json:"provider"`
	Model    string     `json:"model"`
}

// IsConfigured returns true if embedding settings are properly configured
// Note: API key availability is checked at the infrastructure layer (config package)
func (e *EmbeddingSettings) IsConfigured() bool {
	return e.Provider != "" && e.Model != ""
}

// LLMSettings configures the LLM service
// API keys and base URLs come from environment variables, not stored here
type LLMSettings struct {
	Provider AIProvider `json:"provider"`
	Model    string     `json:"model"`
}

// IsConfigured returns true if LLM settings are properly configured
// Note: API key availability is checked at the infrastructure layer (config package)
func (l *LLMSettings) IsConfigured() bool {
	return l.Provider != "" && l.Model != ""
}

// RequiresAPIKey returns true if this provider requires an API key
func (p AIProvider) RequiresAPIKey() bool {
	switch p {
	case AIProviderOllama:
		return false // Self-hosted, no API key needed
	default:
		return true
	}
}

// IsValid returns true if this is a known provider
func (p AIProvider) IsValid() bool {
	switch p {
	case AIProviderOpenAI, AIProviderAnthropic, AIProviderOllama, AIProviderCohere, AIProviderVoyage:
		return true
	default:
		return false
	}
}

// Validate checks if AISettings are valid
func (s *AISettings) Validate() error {
	if s.Embedding.Provider != "" && !s.Embedding.Provider.IsValid() {
		return ErrInvalidProvider
	}
	if s.LLM.Provider != "" && !s.LLM.Provider.IsValid() {
		return ErrInvalidProvider
	}
	return nil
}

// AIModelInfo describes a specific model offered by a provider
type AIModelInfo struct {
	ID         string `json:"id"`           // Model identifier (e.g., "text-embedding-3-small")
	Name       string `json:"name"`         // Display name
	Dimensions int    `json:"dimensions,omitempty"` // For embedding models only
}

// AIProviderInfo provides metadata about an AI provider
type AIProviderInfo struct {
	ID               string        `json:"id"`                  // Provider identifier (matches AIProvider)
	Name             string        `json:"name"`                // Display name
	Models           []AIModelInfo `json:"models"`              // Available models
	RequiresAPIKey   bool          `json:"requires_api_key"`    // Whether API key is needed
	RequiresBaseURL  bool          `json:"requires_base_url"`   // Whether base URL is needed (e.g., Ollama)
	APIKeyURL        string        `json:"api_key_url,omitempty"` // URL to get API key
}
