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
	if !settings.SemanticSearchEnabled {
		t.Error("expected SemanticSearchEnabled to be true")
	}
	if !settings.AutoSuggestEnabled {
		t.Error("expected AutoSuggestEnabled to be true")
	}
	if settings.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSettings(t *testing.T) {
	// Note: AI configuration (provider, model, endpoint) is now in AISettings
	settings := &Settings{
		TeamID:                "team-123",
		DefaultSearchMode:     SearchModeTextOnly,
		ResultsPerPage:        10,
		MaxResultsPerPage:     50,
		SyncIntervalMinutes:   30,
		SyncEnabled:           false,
		SemanticSearchEnabled: false,
		AutoSuggestEnabled:    true,
	}

	if settings.DefaultSearchMode != SearchModeTextOnly {
		t.Errorf("expected DefaultSearchMode text, got %s", settings.DefaultSearchMode)
	}
	if settings.SyncEnabled {
		t.Error("expected SyncEnabled to be false")
	}
	if settings.SemanticSearchEnabled {
		t.Error("expected SemanticSearchEnabled to be false")
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
