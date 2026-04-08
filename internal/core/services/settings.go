package services

import (
	"context"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// Ensure settingsService implements SettingsService
var _ driving.SettingsService = (*settingsService)(nil)

// settingsService implements the SettingsService interface
type settingsService struct {
	settingsStore  driven.SettingsStore
	aiFactory      driven.AIServiceFactory
	configProvider driven.ConfigProvider
	services       *runtime.Services
	teamID         string
}

// NewSettingsService creates a new SettingsService
func NewSettingsService(
	settingsStore driven.SettingsStore,
	aiFactory driven.AIServiceFactory,
	configProvider driven.ConfigProvider,
	services *runtime.Services,
	teamID string,
) driving.SettingsService {
	return &settingsService{
		settingsStore:  settingsStore,
		aiFactory:      aiFactory,
		configProvider: configProvider,
		services:       services,
		teamID:         teamID,
	}
}

// Get retrieves the current settings
func (s *settingsService) Get(ctx context.Context) (*domain.Settings, error) {
	return s.settingsStore.GetSettings(ctx, s.teamID)
}

// Update updates settings (admin only)
func (s *settingsService) Update(ctx context.Context, updaterID string, req driving.UpdateSettingsRequest) (*domain.Settings, error) {
	settings, err := s.settingsStore.GetSettings(ctx, s.teamID)
	if err != nil {
		// If settings don't exist, create defaults
		settings = domain.DefaultSettings(s.teamID)
	}

	// Apply updates (AI settings are managed via UpdateAISettings)
	if req.DefaultSearchMode != nil {
		settings.DefaultSearchMode = *req.DefaultSearchMode
	}
	if req.ResultsPerPage != nil {
		settings.ResultsPerPage = *req.ResultsPerPage
	}
	if req.SyncIntervalMinutes != nil {
		settings.SyncIntervalMinutes = *req.SyncIntervalMinutes
	}
	if req.SyncEnabled != nil {
		settings.SyncEnabled = *req.SyncEnabled
	}
	if req.SyncExclusions != nil {
		settings.SyncExclusions = req.SyncExclusions
	}
	settings.UpdatedAt = time.Now()
	settings.UpdatedBy = updaterID

	if err := s.settingsStore.SaveSettings(ctx, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// GetAISettings retrieves the current AI configuration
func (s *settingsService) GetAISettings(ctx context.Context) (*domain.AISettings, error) {
	return s.settingsStore.GetAISettings(ctx, s.teamID)
}

// UpdateAISettings updates AI configuration and hot-reloads services
func (s *settingsService) UpdateAISettings(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
	// Get current settings or create new
	aiSettings, err := s.settingsStore.GetAISettings(ctx, s.teamID)
	if err != nil {
		aiSettings = &domain.AISettings{TeamID: s.teamID}
	}

	// Update embedding settings if provided
	if req.Embedding != nil {
		// Validate that the provider is configured in environment
		if !s.configProvider.IsAIConfigured(req.Embedding.Provider) {
			return nil, domain.ErrInvalidProvider
		}

		aiSettings.Embedding = domain.EmbeddingSettings{
			Provider: req.Embedding.Provider,
			Model:    req.Embedding.Model,
		}
	}

	// Update LLM settings if provided
	if req.LLM != nil {
		// Validate that the provider is configured in environment
		if !s.configProvider.IsAIConfigured(req.LLM.Provider) {
			return nil, domain.ErrInvalidProvider
		}

		aiSettings.LLM = domain.LLMSettings{
			Provider: req.LLM.Provider,
			Model:    req.LLM.Model,
		}
	}

	// Validate domain constraints
	if err := aiSettings.Validate(); err != nil {
		return nil, err
	}

	aiSettings.UpdatedAt = time.Now()

	// Save to persistent store (no API keys stored)
	if err := s.settingsStore.SaveAISettings(ctx, s.teamID, aiSettings); err != nil {
		return nil, err
	}

	// Hot-reload services
	status := &driving.AISettingsStatus{}

	// Create and set embedding service
	if aiSettings.Embedding.IsConfigured() {
		// Get credentials from config provider
		credentials := s.configProvider.GetAICredentials(aiSettings.Embedding.Provider)
		if credentials == nil {
			status.Embedding = driving.AIServiceStatus{Available: false}
		} else {
			embSvc, err := s.aiFactory.CreateEmbeddingService(&aiSettings.Embedding, credentials)
			if err != nil {
				// Log but continue - service will be unavailable
				status.Embedding = driving.AIServiceStatus{Available: false}
			} else if err := s.services.ValidateAndSetEmbedding(ctx, embSvc); err != nil {
				status.Embedding = driving.AIServiceStatus{Available: false}
			} else {
				status.Embedding = driving.AIServiceStatus{
					Available: true,
					Provider:  aiSettings.Embedding.Provider,
					Model:     aiSettings.Embedding.Model,
				}
			}
		}
	} else {
		// Explicitly disable
		s.services.SetEmbeddingService(nil)
		status.Embedding = driving.AIServiceStatus{Available: false}
	}

	// Create and set LLM service
	if aiSettings.LLM.IsConfigured() {
		// Get credentials from config provider
		credentials := s.configProvider.GetAICredentials(aiSettings.LLM.Provider)
		if credentials == nil {
			status.LLM = driving.AIServiceStatus{Available: false}
		} else {
			llmSvc, err := s.aiFactory.CreateLLMService(&aiSettings.LLM, credentials)
			if err != nil {
				status.LLM = driving.AIServiceStatus{Available: false}
			} else if err := s.services.ValidateAndSetLLM(ctx, llmSvc); err != nil {
				status.LLM = driving.AIServiceStatus{Available: false}
			} else {
				status.LLM = driving.AIServiceStatus{
					Available: true,
					Provider:  aiSettings.LLM.Provider,
					Model:     aiSettings.LLM.Model,
				}
			}
		}
	} else {
		s.services.SetLLMService(nil)
		status.LLM = driving.AIServiceStatus{Available: false}
	}

	// Update effective search mode
	status.EffectiveSearchMode = s.services.Config().EffectiveSearchMode()

	return status, nil
}

// RestoreAIServices restores AI services from persisted settings on startup.
// This must be called during application boot to ensure embedding/LLM services
// are available without requiring the user to re-configure via the API.
func (s *settingsService) RestoreAIServices(ctx context.Context) error {
	aiSettings, err := s.settingsStore.GetAISettings(ctx, s.teamID)
	if err != nil || aiSettings == nil {
		// No saved settings — nothing to restore
		return nil
	}

	// Restore embedding service
	if aiSettings.Embedding.IsConfigured() {
		credentials := s.configProvider.GetAICredentials(aiSettings.Embedding.Provider)
		if credentials != nil {
			embSvc, err := s.aiFactory.CreateEmbeddingService(&aiSettings.Embedding, credentials)
			if err == nil {
				_ = s.services.ValidateAndSetEmbedding(ctx, embSvc)
			}
		}
	}

	// Restore LLM service
	if aiSettings.LLM.IsConfigured() {
		credentials := s.configProvider.GetAICredentials(aiSettings.LLM.Provider)
		if credentials != nil {
			llmSvc, err := s.aiFactory.CreateLLMService(&aiSettings.LLM, credentials)
			if err == nil {
				_ = s.services.ValidateAndSetLLM(ctx, llmSvc)
			}
		}
	}

	return nil
}

// GetAIStatus returns the current status of AI services
func (s *settingsService) GetAIStatus(ctx context.Context) (*driving.AISettingsStatus, error) {
	aiSettings, _ := s.settingsStore.GetAISettings(ctx, s.teamID)

	status := &driving.AISettingsStatus{
		EffectiveSearchMode: s.services.Config().EffectiveSearchMode(),
	}

	// Embedding status
	embSvc := s.services.EmbeddingService()
	if embSvc != nil {
		status.Embedding = driving.AIServiceStatus{
			Available: true,
			Model:     embSvc.Model(),
		}
		if aiSettings != nil {
			status.Embedding.Provider = aiSettings.Embedding.Provider
		}
	}

	// LLM status
	llmSvc := s.services.LLMService()
	if llmSvc != nil {
		status.LLM = driving.AIServiceStatus{
			Available: true,
			Model:     llmSvc.Model(),
		}
		if aiSettings != nil {
			status.LLM.Provider = aiSettings.LLM.Provider
		}
	}

	return status, nil
}

// TestConnection tests the AI provider connection
func (s *settingsService) TestConnection(ctx context.Context) error {
	embSvc := s.services.EmbeddingService()
	if embSvc != nil {
		if err := embSvc.HealthCheck(ctx); err != nil {
			return err
		}
	}

	llmSvc := s.services.LLMService()
	if llmSvc != nil {
		if err := llmSvc.Ping(ctx); err != nil {
			return err
		}
	}

	return nil
}

// GetAIProviders returns static metadata about available AI providers
func (s *settingsService) GetAIProviders(ctx context.Context) (*driving.AIProvidersResponse, error) {
	return &driving.AIProvidersResponse{
		Embedding: buildEmbeddingProviders(),
		LLM:       buildLLMProviders(),
	}, nil
}

// buildEmbeddingProviders returns static metadata for embedding providers
func buildEmbeddingProviders() []domain.AIProviderInfo {
	return []domain.AIProviderInfo{
		{
			ID:   string(domain.AIProviderOpenAI),
			Name: "OpenAI",
			Models: []domain.AIModelInfo{
				{
					ID:         "text-embedding-3-small",
					Name:       "Text Embedding 3 Small",
					Dimensions: 1536,
				},
				{
					ID:         "text-embedding-3-large",
					Name:       "Text Embedding 3 Large",
					Dimensions: 3072,
				},
				{
					ID:         "text-embedding-ada-002",
					Name:       "Text Embedding Ada 002",
					Dimensions: 1536,
				},
			},
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			APIKeyURL:       "https://platform.openai.com/api-keys",
		},
		{
			ID:   string(domain.AIProviderOllama),
			Name: "Ollama",
			Models: []domain.AIModelInfo{
				{
					ID:         "nomic-embed-text",
					Name:       "Nomic Embed Text",
					Dimensions: 768,
				},
				{
					ID:         "mxbai-embed-large",
					Name:       "MixedBread AI Embed Large",
					Dimensions: 1024,
				},
			},
			RequiresAPIKey:  false,
			RequiresBaseURL: true,
		},
		{
			ID:   string(domain.AIProviderCohere),
			Name: "Cohere",
			Models: []domain.AIModelInfo{
				{
					ID:         "embed-english-v3.0",
					Name:       "Embed English v3.0",
					Dimensions: 1024,
				},
				{
					ID:         "embed-multilingual-v3.0",
					Name:       "Embed Multilingual v3.0",
					Dimensions: 1024,
				},
			},
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			APIKeyURL:       "https://dashboard.cohere.com/api-keys",
		},
		{
			ID:   string(domain.AIProviderVoyage),
			Name: "Voyage AI",
			Models: []domain.AIModelInfo{
				{
					ID:         "voyage-2",
					Name:       "Voyage 2",
					Dimensions: 1024,
				},
				{
					ID:         "voyage-large-2",
					Name:       "Voyage Large 2",
					Dimensions: 1536,
				},
				{
					ID:         "voyage-code-2",
					Name:       "Voyage Code 2",
					Dimensions: 1536,
				},
			},
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			APIKeyURL:       "https://dash.voyageai.com/api-keys",
		},
	}
}

// buildLLMProviders returns static metadata for LLM providers
func buildLLMProviders() []domain.AIProviderInfo {
	return []domain.AIProviderInfo{
		{
			ID:   string(domain.AIProviderOpenAI),
			Name: "OpenAI",
			Models: []domain.AIModelInfo{
				{
					ID:   "gpt-4o",
					Name: "GPT-4o",
				},
				{
					ID:   "gpt-4o-mini",
					Name: "GPT-4o Mini",
				},
				{
					ID:   "gpt-4-turbo",
					Name: "GPT-4 Turbo",
				},
				{
					ID:   "gpt-3.5-turbo",
					Name: "GPT-3.5 Turbo",
				},
			},
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			APIKeyURL:       "https://platform.openai.com/api-keys",
		},
		{
			ID:   string(domain.AIProviderAnthropic),
			Name: "Anthropic",
			Models: []domain.AIModelInfo{
				{
					ID:   "claude-3-5-sonnet-20241022",
					Name: "Claude 3.5 Sonnet",
				},
				{
					ID:   "claude-3-opus-20240229",
					Name: "Claude 3 Opus",
				},
				{
					ID:   "claude-3-haiku-20240307",
					Name: "Claude 3 Haiku",
				},
			},
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			APIKeyURL:       "https://console.anthropic.com/settings/keys",
		},
		{
			ID:   string(domain.AIProviderOllama),
			Name: "Ollama",
			Models: []domain.AIModelInfo{
				{
					ID:   "llama3.2",
					Name: "Llama 3.2",
				},
				{
					ID:   "llama3.1",
					Name: "Llama 3.1",
				},
				{
					ID:   "mistral",
					Name: "Mistral",
				},
				{
					ID:   "qwen2.5",
					Name: "Qwen 2.5",
				},
			},
			RequiresAPIKey:  false,
			RequiresBaseURL: true,
		},
	}
}
