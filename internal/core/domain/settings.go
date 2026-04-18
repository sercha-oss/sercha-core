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

// SyncExclusionSettings holds sync exclusion patterns configuration
type SyncExclusionSettings struct {
	// EnabledPatterns are the default patterns that are currently enabled
	EnabledPatterns []string `json:"enabled_patterns"`

	// DisabledPatterns are the default patterns that have been toggled off
	DisabledPatterns []string `json:"disabled_patterns"`

	// CustomPatterns are user-defined exclusion patterns
	CustomPatterns []string `json:"custom_patterns"`

	// MimeExclusions are MIME type patterns to exclude (e.g., "image/*", "font/*")
	MimeExclusions []string `json:"mime_exclusions"`
}

// GetActivePatterns returns all active exclusion patterns (enabled defaults + custom)
func (s *SyncExclusionSettings) GetActivePatterns() []string {
	if s == nil {
		return []string{}
	}

	// Combine enabled patterns and custom patterns
	active := make([]string, 0, len(s.EnabledPatterns)+len(s.CustomPatterns))
	active = append(active, s.EnabledPatterns...)
	active = append(active, s.CustomPatterns...)
	return active
}

// GetActiveMimeExclusions returns all active MIME type exclusion patterns
func (s *SyncExclusionSettings) GetActiveMimeExclusions() []string {
	if s == nil {
		return []string{}
	}
	return s.MimeExclusions
}

// HasPatterns returns true if there are any active exclusion patterns
func (s *SyncExclusionSettings) HasPatterns() bool {
	if s == nil {
		return false
	}
	return len(s.EnabledPatterns) > 0 || len(s.CustomPatterns) > 0
}

// HasMimeExclusions returns true if there are any MIME exclusion patterns
func (s *SyncExclusionSettings) HasMimeExclusions() bool {
	if s == nil {
		return false
	}
	return len(s.MimeExclusions) > 0
}

// Settings holds team-wide configuration
// Note: AI configuration (provider, model, API keys) is managed via AISettings
// and the /settings/ai endpoint, not here.
// Note: Semantic/vector search is now controlled via CapabilityPreferences.
type Settings struct {
	TeamID string `json:"team_id"`

	// Search Defaults
	DefaultSearchMode SearchMode `json:"default_search_mode"`
	ResultsPerPage    int        `json:"results_per_page"`
	MaxResultsPerPage int        `json:"max_results_per_page"`

	// Sync Configuration
	SyncIntervalMinutes int                    `json:"sync_interval_minutes"`
	SyncEnabled         bool                   `json:"sync_enabled"`
	SyncExclusions      *SyncExclusionSettings `json:"sync_exclusions,omitempty"`

	// Metadata
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"` // User ID
}

// DefaultSyncExclusions returns common file/folder patterns to exclude from sync
func DefaultSyncExclusions() *SyncExclusionSettings {
	return &SyncExclusionSettings{
		EnabledPatterns: []string{
			// Version control
			".git/",
			".svn/",
			".hg/",
			// Dependencies
			"node_modules/",
			"vendor/",
			"venv/",
			".venv/",
			// Build artifacts
			"build/",
			"dist/",
			"target/",
			"out/",
			"bin/",
			// IDE/Editor
			".idea/",
			".vscode/",
			".vs/",
			// OS files
			".DS_Store",
			"Thumbs.db",
			// Temporary files
			"*.tmp",
			"*.temp",
			"*.log",
			// Archives
			"*.zip",
			"*.tar.gz",
			"*.rar",
			// Media (large files)
			"*.mp4",
			"*.mov",
			"*.avi",
			"*.mp3",
			"*.wav",
			// Images
			"*.png",
			"*.jpg",
			"*.jpeg",
			"*.gif",
			"*.webp",
			"*.svg",
			"*.ico",
			"*.bmp",
		},
		DisabledPatterns: []string{},
		CustomPatterns:   []string{},
		MimeExclusions: []string{
			// Images (binary content)
			"image/*",
			// Fonts (binary content)
			"font/*",
			// Archives and compressed files
			"application/zip",
			"application/x-tar",
			"application/gzip",
			"application/vnd.rar",
			"application/x-7z-compressed",
			// Executables and libraries
			"application/x-msdownload",
			"application/x-sharedlib",
			// Audio/video (typically not searchable text)
			"audio/*",
			"video/*",
		},
	}
}

// DefaultSettings returns sensible defaults for a new team
func DefaultSettings(teamID string) *Settings {
	return &Settings{
		TeamID:              teamID,
		DefaultSearchMode:   SearchModeHybrid,
		ResultsPerPage:      20,
		MaxResultsPerPage:   100,
		SyncIntervalMinutes: 60,
		SyncEnabled:         true,
		SyncExclusions:      DefaultSyncExclusions(),
		UpdatedAt:           time.Now(),
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
	ID         string `json:"id"`                   // Model identifier (e.g., "text-embedding-3-small")
	Name       string `json:"name"`                 // Display name
	Dimensions int    `json:"dimensions,omitempty"` // For embedding models only
}

// AIProviderInfo provides metadata about an AI provider
type AIProviderInfo struct {
	ID              string        `json:"id"`                    // Provider identifier (matches AIProvider)
	Name            string        `json:"name"`                  // Display name
	Models          []AIModelInfo `json:"models"`                // Available models
	RequiresAPIKey  bool          `json:"requires_api_key"`      // Whether API key is needed
	RequiresBaseURL bool          `json:"requires_base_url"`     // Whether base URL is needed (e.g., Ollama)
	APIKeyURL       string        `json:"api_key_url,omitempty"` // URL to get API key
}
