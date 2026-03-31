package services

import (
	"context"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
	"github.com/custodia-labs/sercha-core/internal/runtime"
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
	if req.SemanticSearchEnabled != nil {
		settings.SemanticSearchEnabled = *req.SemanticSearchEnabled
	}
	if req.AutoSuggestEnabled != nil {
		settings.AutoSuggestEnabled = *req.AutoSuggestEnabled
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
