package domain

import (
	"testing"
)

func TestAIProviderConstants(t *testing.T) {
	tests := []struct {
		provider AIProvider
		expected string
	}{
		{AIProviderOpenAI, "openai"},
		{AIProviderAnthropic, "anthropic"},
		{AIProviderOllama, "ollama"},
		{AIProviderCohere, "cohere"},
		{AIProviderVoyage, "voyage"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.provider))
			}
		})
	}
}

func TestDefaultSettings(t *testing.T) {
	teamID := "team-123"
	settings := DefaultSettings(teamID)

	if settings.TeamID != teamID {
		t.Errorf("expected TeamID %s, got %s", teamID, settings.TeamID)
	}
	// Note: AI configuration is now in AISettings, not Settings
	if settings.DefaultSearchMode != SearchModeHybrid {
		t.Errorf("expected DefaultSearchMode hybrid, got %s", settings.DefaultSearchMode)
	}
	if settings.ResultsPerPage != 20 {
		t.Errorf("expected ResultsPerPage 20, got %d", settings.ResultsPerPage)
	}
	if settings.MaxResultsPerPage != 100 {
		t.Errorf("expected MaxResultsPerPage 100, got %d", settings.MaxResultsPerPage)
	}
	if settings.SyncIntervalMinutes != 60 {
		t.Errorf("expected SyncIntervalMinutes 60, got %d", settings.SyncIntervalMinutes)
	}
	if !settings.SyncEnabled {
		t.Error("expected SyncEnabled to be true")
	}
	if settings.SyncExclusions == nil {
		t.Error("expected SyncExclusions to be initialized")
	}
	if settings.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSettings(t *testing.T) {
	// Note: AI configuration (provider, model, endpoint) is now in AISettings
	// Note: Semantic/vector search is now controlled via CapabilityPreferences
	settings := &Settings{
		TeamID:              "team-123",
		DefaultSearchMode:   SearchModeTextOnly,
		ResultsPerPage:      10,
		MaxResultsPerPage:   50,
		SyncIntervalMinutes: 30,
		SyncEnabled:         false,
	}

	if settings.DefaultSearchMode != SearchModeTextOnly {
		t.Errorf("expected DefaultSearchMode text, got %s", settings.DefaultSearchMode)
	}
	if settings.SyncEnabled {
		t.Error("expected SyncEnabled to be false")
	}
	if settings.ResultsPerPage != 10 {
		t.Errorf("expected ResultsPerPage 10, got %d", settings.ResultsPerPage)
	}
}

// TestAPIKeyConfig removed - API keys no longer stored in domain models

func TestDefaultEmbeddingConfig(t *testing.T) {
	config := DefaultEmbeddingConfig()

	if config.Provider != AIProviderOpenAI {
		t.Errorf("expected Provider openai, got %s", config.Provider)
	}
	if config.Model != "text-embedding-3-small" {
		t.Errorf("expected Model text-embedding-3-small, got %s", config.Model)
	}
	if config.Dimensions != 1536 {
		t.Errorf("expected Dimensions 1536, got %d", config.Dimensions)
	}
	if config.BatchSize != 100 {
		t.Errorf("expected BatchSize 100, got %d", config.BatchSize)
	}
}

func TestEmbeddingConfig(t *testing.T) {
	config := &EmbeddingConfig{
		Provider:   AIProviderVoyage,
		Model:      "voyage-large-2",
		Dimensions: 1024,
		BatchSize:  50,
	}

	if config.Provider != AIProviderVoyage {
		t.Errorf("expected Provider voyage, got %s", config.Provider)
	}
	if config.Model != "voyage-large-2" {
		t.Errorf("expected Model voyage-large-2, got %s", config.Model)
	}
	if config.Dimensions != 1024 {
		t.Errorf("expected Dimensions 1024, got %d", config.Dimensions)
	}
	if config.BatchSize != 50 {
		t.Errorf("expected BatchSize 50, got %d", config.BatchSize)
	}
}

func TestEmbeddingSettings_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		settings EmbeddingSettings
		expected bool
	}{
		{
			name:     "empty provider",
			settings: EmbeddingSettings{Provider: "", Model: "test"},
			expected: false,
		},
		{
			name:     "provider without model",
			settings: EmbeddingSettings{Provider: AIProviderOpenAI, Model: ""},
			expected: false,
		},
		{
			name:     "openai with model",
			settings: EmbeddingSettings{Provider: AIProviderOpenAI, Model: "text-embedding-3-small"},
			expected: true,
		},
		{
			name:     "ollama with model",
			settings: EmbeddingSettings{Provider: AIProviderOllama, Model: "nomic"},
			expected: true,
		},
		{
			name:     "voyage with model",
			settings: EmbeddingSettings{Provider: AIProviderVoyage, Model: "voyage-2"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.settings.IsConfigured()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLLMSettings_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		settings LLMSettings
		expected bool
	}{
		{
			name:     "empty provider",
			settings: LLMSettings{Provider: "", Model: "test"},
			expected: false,
		},
		{
			name:     "provider without model",
			settings: LLMSettings{Provider: AIProviderOpenAI, Model: ""},
			expected: false,
		},
		{
			name:     "openai with model",
			settings: LLMSettings{Provider: AIProviderOpenAI, Model: "gpt-4"},
			expected: true,
		},
		{
			name:     "anthropic with model",
			settings: LLMSettings{Provider: AIProviderAnthropic, Model: "claude-3-5-sonnet"},
			expected: true,
		},
		{
			name:     "ollama with model",
			settings: LLMSettings{Provider: AIProviderOllama, Model: "llama3"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.settings.IsConfigured()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAIProvider_RequiresAPIKey(t *testing.T) {
	tests := []struct {
		provider AIProvider
		requires bool
	}{
		{AIProviderOpenAI, true},
		{AIProviderAnthropic, true},
		{AIProviderCohere, true},
		{AIProviderVoyage, true},
		{AIProviderOllama, false}, // Self-hosted
		{"unknown", true},         // Default to requiring key
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := tt.provider.RequiresAPIKey()
			if result != tt.requires {
				t.Errorf("expected %v, got %v", tt.requires, result)
			}
		})
	}
}

func TestAIProvider_IsValid(t *testing.T) {
	tests := []struct {
		provider AIProvider
		valid    bool
	}{
		{AIProviderOpenAI, true},
		{AIProviderAnthropic, true},
		{AIProviderOllama, true},
		{AIProviderCohere, true},
		{AIProviderVoyage, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		name := string(tt.provider)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			result := tt.provider.IsValid()
			if result != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, result)
			}
		})
	}
}

func TestAISettings_Validate(t *testing.T) {
	tests := []struct {
		name     string
		settings AISettings
		wantErr  bool
	}{
		{
			name:     "empty settings (valid)",
			settings: AISettings{},
			wantErr:  false,
		},
		{
			name: "valid embedding provider",
			settings: AISettings{
				Embedding: EmbeddingSettings{Provider: AIProviderOpenAI},
			},
			wantErr: false,
		},
		{
			name: "valid llm provider",
			settings: AISettings{
				LLM: LLMSettings{Provider: AIProviderAnthropic},
			},
			wantErr: false,
		},
		{
			name: "invalid embedding provider",
			settings: AISettings{
				Embedding: EmbeddingSettings{Provider: "invalid-provider"},
			},
			wantErr: true,
		},
		{
			name: "invalid llm provider",
			settings: AISettings{
				LLM: LLMSettings{Provider: "invalid-provider"},
			},
			wantErr: true,
		},
		{
			name: "both valid",
			settings: AISettings{
				Embedding: EmbeddingSettings{Provider: AIProviderOpenAI},
				LLM:       LLMSettings{Provider: AIProviderOllama},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAISettings(t *testing.T) {
	settings := &AISettings{
		TeamID: "team-1",
		Embedding: EmbeddingSettings{
			Provider: AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
		LLM: LLMSettings{
			Provider: AIProviderAnthropic,
			Model:    "claude-3-5-sonnet",
		},
	}

	if settings.TeamID != "team-1" {
		t.Errorf("expected team-1, got %s", settings.TeamID)
	}
	if !settings.Embedding.IsConfigured() {
		t.Error("expected embedding to be configured")
	}
	if !settings.LLM.IsConfigured() {
		t.Error("expected LLM to be configured")
	}
}

func TestAIModelInfo(t *testing.T) {
	model := AIModelInfo{
		ID:         "text-embedding-3-small",
		Name:       "Text Embedding 3 Small",
		Dimensions: 1536,
	}

	if model.ID != "text-embedding-3-small" {
		t.Errorf("expected ID text-embedding-3-small, got %s", model.ID)
	}
	if model.Name != "Text Embedding 3 Small" {
		t.Errorf("expected Name 'Text Embedding 3 Small', got %s", model.Name)
	}
	if model.Dimensions != 1536 {
		t.Errorf("expected Dimensions 1536, got %d", model.Dimensions)
	}
}

func TestAIProviderInfo(t *testing.T) {
	provider := AIProviderInfo{
		ID:   "openai",
		Name: "OpenAI",
		Models: []AIModelInfo{
			{ID: "text-embedding-3-small", Name: "Text Embedding 3 Small", Dimensions: 1536},
			{ID: "text-embedding-3-large", Name: "Text Embedding 3 Large", Dimensions: 3072},
		},
		RequiresAPIKey:  true,
		RequiresBaseURL: false,
		APIKeyURL:       "https://platform.openai.com/api-keys",
	}

	if provider.ID != "openai" {
		t.Errorf("expected ID openai, got %s", provider.ID)
	}
	if provider.Name != "OpenAI" {
		t.Errorf("expected Name OpenAI, got %s", provider.Name)
	}
	if len(provider.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(provider.Models))
	}
	if !provider.RequiresAPIKey {
		t.Error("expected RequiresAPIKey to be true")
	}
	if provider.RequiresBaseURL {
		t.Error("expected RequiresBaseURL to be false")
	}
	if provider.APIKeyURL != "https://platform.openai.com/api-keys" {
		t.Errorf("unexpected APIKeyURL: %s", provider.APIKeyURL)
	}
}

func TestDefaultSyncExclusions(t *testing.T) {
	exclusions := DefaultSyncExclusions()

	if exclusions == nil {
		t.Fatal("expected DefaultSyncExclusions to return non-nil value")
	}

	if len(exclusions.EnabledPatterns) == 0 {
		t.Error("expected EnabledPatterns to have default values")
	}

	if len(exclusions.DisabledPatterns) != 0 {
		t.Errorf("expected DisabledPatterns to be empty, got %d", len(exclusions.DisabledPatterns))
	}

	if len(exclusions.CustomPatterns) != 0 {
		t.Errorf("expected CustomPatterns to be empty, got %d", len(exclusions.CustomPatterns))
	}

	// Check for some common patterns
	hasGit := false
	hasNodeModules := false
	for _, pattern := range exclusions.EnabledPatterns {
		if pattern == ".git/" {
			hasGit = true
		}
		if pattern == "node_modules/" {
			hasNodeModules = true
		}
	}

	if !hasGit {
		t.Error("expected EnabledPatterns to include .git/")
	}
	if !hasNodeModules {
		t.Error("expected EnabledPatterns to include node_modules/")
	}
}

func TestSyncExclusionSettings_GetActivePatterns(t *testing.T) {
	tests := []struct {
		name      string
		exclusion *SyncExclusionSettings
		expected  int
	}{
		{
			name:      "nil settings",
			exclusion: nil,
			expected:  0,
		},
		{
			name: "empty settings",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{},
			},
			expected: 0,
		},
		{
			name: "only enabled patterns",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{".git/", "node_modules/"},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{},
			},
			expected: 2,
		},
		{
			name: "only custom patterns",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{"*.secret", "private/"},
			},
			expected: 2,
		},
		{
			name: "enabled and custom patterns",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{".git/", "node_modules/"},
				DisabledPatterns: []string{"build/"},
				CustomPatterns:   []string{"*.secret", "private/"},
			},
			expected: 4, // 2 enabled + 2 custom (disabled patterns not included)
		},
		{
			name:      "default settings",
			exclusion: DefaultSyncExclusions(),
			expected:  len(DefaultSyncExclusions().EnabledPatterns),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active := tt.exclusion.GetActivePatterns()
			if len(active) != tt.expected {
				t.Errorf("expected %d active patterns, got %d", tt.expected, len(active))
			}
		})
	}
}

func TestSyncExclusionSettings_HasPatterns(t *testing.T) {
	tests := []struct {
		name      string
		exclusion *SyncExclusionSettings
		expected  bool
	}{
		{
			name:      "nil settings",
			exclusion: nil,
			expected:  false,
		},
		{
			name: "empty settings",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{},
			},
			expected: false,
		},
		{
			name: "has enabled patterns",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{".git/"},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{},
			},
			expected: true,
		},
		{
			name: "has custom patterns only",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{},
				DisabledPatterns: []string{},
				CustomPatterns:   []string{"*.secret"},
			},
			expected: true,
		},
		{
			name: "has disabled patterns only",
			exclusion: &SyncExclusionSettings{
				EnabledPatterns:  []string{},
				DisabledPatterns: []string{"build/"},
				CustomPatterns:   []string{},
			},
			expected: false, // Disabled patterns don't count as active
		},
		{
			name:      "default settings",
			exclusion: DefaultSyncExclusions(),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exclusion.HasPatterns()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSyncExclusionSettings_GetActiveMimeExclusions(t *testing.T) {
	tests := []struct {
		name      string
		exclusion *SyncExclusionSettings
		expected  int
	}{
		{
			name:      "nil settings",
			exclusion: nil,
			expected:  0,
		},
		{
			name: "empty mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{},
			},
			expected: 0,
		},
		{
			name: "with mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{"image/*", "font/*"},
			},
			expected: 2,
		},
		{
			name: "with multiple mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{
					"image/*",
					"font/*",
					"audio/*",
					"video/*",
					"application/zip",
				},
			},
			expected: 5,
		},
		{
			name:      "default settings",
			exclusion: DefaultSyncExclusions(),
			expected:  len(DefaultSyncExclusions().MimeExclusions),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active := tt.exclusion.GetActiveMimeExclusions()
			if len(active) != tt.expected {
				t.Errorf("expected %d active MIME exclusions, got %d", tt.expected, len(active))
			}
		})
	}
}

func TestSyncExclusionSettings_HasMimeExclusions(t *testing.T) {
	tests := []struct {
		name      string
		exclusion *SyncExclusionSettings
		expected  bool
	}{
		{
			name:      "nil settings",
			exclusion: nil,
			expected:  false,
		},
		{
			name: "empty mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{},
			},
			expected: false,
		},
		{
			name: "with mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{"image/*"},
			},
			expected: true,
		},
		{
			name: "with multiple mime exclusions",
			exclusion: &SyncExclusionSettings{
				MimeExclusions: []string{"image/*", "font/*"},
			},
			expected: true,
		},
		{
			name:      "default settings",
			exclusion: DefaultSyncExclusions(),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exclusion.HasMimeExclusions()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultSyncExclusions_MimeExclusions(t *testing.T) {
	exclusions := DefaultSyncExclusions()

	if exclusions == nil {
		t.Fatal("expected DefaultSyncExclusions to return non-nil value")
	}

	if len(exclusions.MimeExclusions) == 0 {
		t.Error("expected MimeExclusions to have default values")
	}

	// Check for common MIME patterns
	hasImageWildcard := false
	hasFontWildcard := false
	hasAudioWildcard := false
	hasVideoWildcard := false
	hasZip := false

	for _, pattern := range exclusions.MimeExclusions {
		switch pattern {
		case "image/*":
			hasImageWildcard = true
		case "font/*":
			hasFontWildcard = true
		case "audio/*":
			hasAudioWildcard = true
		case "video/*":
			hasVideoWildcard = true
		case "application/zip":
			hasZip = true
		}
	}

	if !hasImageWildcard {
		t.Error("expected MimeExclusions to include image/*")
	}
	if !hasFontWildcard {
		t.Error("expected MimeExclusions to include font/*")
	}
	if !hasAudioWildcard {
		t.Error("expected MimeExclusions to include audio/*")
	}
	if !hasVideoWildcard {
		t.Error("expected MimeExclusions to include video/*")
	}
	if !hasZip {
		t.Error("expected MimeExclusions to include application/zip")
	}
}
