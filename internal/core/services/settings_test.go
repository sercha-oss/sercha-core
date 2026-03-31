package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
	"github.com/custodia-labs/sercha-core/internal/runtime"
)

// mockSettingsStore implements driven.SettingsStore for testing
type mockSettingsStore struct {
	settings   *domain.Settings
	aiSettings *domain.AISettings
	saveErr    error
}

func (m *mockSettingsStore) GetSettings(ctx context.Context, teamID string) (*domain.Settings, error) {
	if m.settings == nil {
		return nil, domain.ErrNotFound
	}
	return m.settings, nil
}

func (m *mockSettingsStore) SaveSettings(ctx context.Context, settings *domain.Settings) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.settings = settings
	return nil
}

func (m *mockSettingsStore) GetAISettings(ctx context.Context, teamID string) (*domain.AISettings, error) {
	if m.aiSettings == nil {
		return &domain.AISettings{TeamID: teamID}, nil
	}
	return m.aiSettings, nil
}

func (m *mockSettingsStore) SaveAISettings(ctx context.Context, teamID string, settings *domain.AISettings) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.aiSettings = settings
	return nil
}

// mockAIFactory implements driven.AIServiceFactory for testing
type mockAIFactory struct {
	embeddingErr error
	llmErr       error
}

func (m *mockAIFactory) CreateEmbeddingService(settings *domain.EmbeddingSettings, credentials *driven.AICredentials) (driven.EmbeddingService, error) {
	if settings == nil || !settings.IsConfigured() {
		return nil, nil
	}
	if m.embeddingErr != nil {
		return nil, m.embeddingErr
	}
	return &mockEmbeddingService{}, nil
}

func (m *mockAIFactory) CreateLLMService(settings *domain.LLMSettings, credentials *driven.AICredentials) (driven.LLMService, error) {
	if settings == nil || !settings.IsConfigured() {
		return nil, nil
	}
	if m.llmErr != nil {
		return nil, m.llmErr
	}
	return &mockLLMService{}, nil
}

// mockEmbeddingService for testing
type mockEmbeddingService struct {
	healthCheckErr error
}

func (m *mockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

func (m *mockEmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return nil, nil
}

func (m *mockEmbeddingService) Dimensions() int {
	return 384
}

func (m *mockEmbeddingService) Model() string {
	return "test-embedding"
}

func (m *mockEmbeddingService) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *mockEmbeddingService) Close() error {
	return nil
}

// mockLLMService for testing
type mockLLMService struct {
	pingErr error
}

func (m *mockLLMService) ExpandQuery(ctx context.Context, query string) ([]string, error) {
	return nil, nil
}

func (m *mockLLMService) Summarise(ctx context.Context, content string, maxLen int) (string, error) {
	return "", nil
}

func (m *mockLLMService) RewriteQuery(ctx context.Context, query string) (string, error) {
	return "", nil
}

func (m *mockLLMService) Model() string {
	return "test-llm"
}

func (m *mockLLMService) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockLLMService) Close() error {
	return nil
}

func TestSettingsService_Get(t *testing.T) {
	store := &mockSettingsStore{
		settings: &domain.Settings{
			TeamID:         "team-1",
			ResultsPerPage: 20,
		},
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	settings, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.TeamID != "team-1" {
		t.Errorf("expected team-1, got %s", settings.TeamID)
	}
}

func TestSettingsService_Update(t *testing.T) {
	store := &mockSettingsStore{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resultsPerPage := 50
	req := driving.UpdateSettingsRequest{
		ResultsPerPage: &resultsPerPage,
	}

	settings, err := svc.Update(context.Background(), "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ResultsPerPage != 50 {
		t.Errorf("expected results per page 50, got %d", settings.ResultsPerPage)
	}
	if settings.UpdatedBy != "user-1" {
		t.Errorf("expected updated by user-1, got %s", settings.UpdatedBy)
	}
}

func TestSettingsService_Update_AllFields(t *testing.T) {
	store := &mockSettingsStore{
		settings: domain.DefaultSettings("team-1"),
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	// Note: AI configuration (provider, model, endpoint) is now managed via UpdateAISettings
	searchMode := domain.SearchModeHybrid
	resultsPerPage := 30
	syncInterval := 120
	syncEnabled := false
	semanticEnabled := true
	autoSuggest := true

	req := driving.UpdateSettingsRequest{
		DefaultSearchMode:     &searchMode,
		ResultsPerPage:        &resultsPerPage,
		SyncIntervalMinutes:   &syncInterval,
		SyncEnabled:           &syncEnabled,
		SemanticSearchEnabled: &semanticEnabled,
		AutoSuggestEnabled:    &autoSuggest,
	}

	settings, err := svc.Update(context.Background(), "admin", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.DefaultSearchMode != domain.SearchModeHybrid {
		t.Errorf("expected hybrid mode, got %s", settings.DefaultSearchMode)
	}
	if settings.ResultsPerPage != 30 {
		t.Errorf("expected 30, got %d", settings.ResultsPerPage)
	}
	if settings.SyncIntervalMinutes != 120 {
		t.Errorf("expected 120, got %d", settings.SyncIntervalMinutes)
	}
}

func TestSettingsService_GetAISettings(t *testing.T) {
	store := &mockSettingsStore{
		aiSettings: &domain.AISettings{
			TeamID: "team-1",
			Embedding: domain.EmbeddingSettings{
				Provider: domain.AIProviderOpenAI,
				Model:    "text-embedding-3-small",
			},
		},
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	aiSettings, err := svc.GetAISettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if aiSettings.Embedding.Provider != domain.AIProviderOpenAI {
		t.Errorf("expected openai provider, got %s", aiSettings.Embedding.Provider)
	}
}

func TestSettingsService_UpdateAISettings(t *testing.T) {
	store := &mockSettingsStore{}
	factory := &mockAIFactory{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	// Configure AI provider credentials
	configProvider.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
		APIKey: "sk-test",
	}
	svc := NewSettingsService(store, factory, configProvider, services, "team-1")

	req := driving.UpdateAISettingsRequest{
		Embedding: &driving.EmbeddingSettingsInput{
			Provider: domain.AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
	}

	status, err := svc.UpdateAISettings(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Embedding.Available {
		t.Error("expected embedding to be available")
	}
}

func TestSettingsService_UpdateAISettings_FactoryError(t *testing.T) {
	store := &mockSettingsStore{}
	factory := &mockAIFactory{
		embeddingErr: errors.New("failed to create service"),
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	// Configure AI provider credentials
	configProvider.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
		APIKey: "sk-test",
	}
	svc := NewSettingsService(store, factory, configProvider, services, "team-1")

	req := driving.UpdateAISettingsRequest{
		Embedding: &driving.EmbeddingSettingsInput{
			Provider: domain.AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
	}

	status, err := svc.UpdateAISettings(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Embedding.Available {
		t.Error("expected embedding to be unavailable when factory fails")
	}
}

func TestSettingsService_UpdateAISettings_DisableService(t *testing.T) {
	store := &mockSettingsStore{}
	factory := &mockAIFactory{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)

	// First set up an embedding service
	mock := &mockEmbeddingService{}
	services.SetEmbeddingService(mock)

	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, factory, configProvider, services, "team-1")

	// Update with empty embedding (should disable)
	req := driving.UpdateAISettingsRequest{
		Embedding: nil, // Not provided = disable
	}

	status, err := svc.UpdateAISettings(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Embedding.Available {
		t.Error("expected embedding to be unavailable after disabling")
	}
}

func TestSettingsService_GetAIStatus(t *testing.T) {
	store := &mockSettingsStore{
		aiSettings: &domain.AISettings{
			TeamID: "team-1",
			Embedding: domain.EmbeddingSettings{
				Provider: domain.AIProviderOpenAI,
			},
		},
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)

	// Set up embedding service
	services.SetEmbeddingService(&mockEmbeddingService{})

	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	status, err := svc.GetAIStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Embedding.Available {
		t.Error("expected embedding to be available")
	}
	if status.Embedding.Model != "test-embedding" {
		t.Errorf("expected test-embedding model, got %s", status.Embedding.Model)
	}
}

func TestSettingsService_TestConnection(t *testing.T) {
	t.Run("no services configured", func(t *testing.T) {
		store := &mockSettingsStore{}
		config := domain.NewRuntimeConfig("postgres")
		services := runtime.NewServices(config)
		configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

		err := svc.TestConnection(context.Background())
		if err != nil {
			t.Errorf("expected no error when no services configured, got %v", err)
		}
	})

	t.Run("embedding healthy", func(t *testing.T) {
		store := &mockSettingsStore{}
		config := domain.NewRuntimeConfig("postgres")
		services := runtime.NewServices(config)
		services.SetEmbeddingService(&mockEmbeddingService{})
		configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

		err := svc.TestConnection(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("embedding unhealthy", func(t *testing.T) {
		store := &mockSettingsStore{}
		config := domain.NewRuntimeConfig("postgres")
		services := runtime.NewServices(config)
		services.SetEmbeddingService(&mockEmbeddingService{healthCheckErr: errors.New("connection failed")})
		configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

		err := svc.TestConnection(context.Background())
		if err == nil {
			t.Error("expected error for unhealthy service")
		}
	})

	t.Run("llm healthy", func(t *testing.T) {
		store := &mockSettingsStore{}
		config := domain.NewRuntimeConfig("postgres")
		services := runtime.NewServices(config)
		services.SetLLMService(&mockLLMService{})
		configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

		err := svc.TestConnection(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("llm unhealthy", func(t *testing.T) {
		store := &mockSettingsStore{}
		config := domain.NewRuntimeConfig("postgres")
		services := runtime.NewServices(config)
		services.SetLLMService(&mockLLMService{pingErr: errors.New("connection failed")})
		configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

		err := svc.TestConnection(context.Background())
		if err == nil {
			t.Error("expected error for unhealthy service")
		}
	})
}

func TestSettingsService_UpdateAISettings_WithLLM(t *testing.T) {
	store := &mockSettingsStore{}
	factory := &mockAIFactory{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, factory, configProvider, services, "team-1")

	// Configure AI credentials
	configProvider.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
		APIKey: "sk-test",
	}

	req := driving.UpdateAISettingsRequest{
		LLM: &driving.LLMSettingsInput{
			Provider: domain.AIProviderOpenAI,
			Model:    "gpt-4o-mini",
		},
	}

	status, err := svc.UpdateAISettings(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.LLM.Available {
		t.Error("expected LLM to be available")
	}
}

func TestSettingsService_UpdateAISettings_SaveError(t *testing.T) {
	store := &mockSettingsStore{
		saveErr: errors.New("database error"),
	}
	factory := &mockAIFactory{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	// Configure AI credentials
	configProvider.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
		APIKey: "sk-test",
	}
	svc := NewSettingsService(store, factory, configProvider, services, "team-1")

	req := driving.UpdateAISettingsRequest{
		Embedding: &driving.EmbeddingSettingsInput{
			Provider: domain.AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
	}

	_, err := svc.UpdateAISettings(context.Background(), req)
	if err == nil {
		t.Error("expected error when save fails")
	}
}

func TestSettingsService_Update_ExistingSettings(t *testing.T) {
	store := &mockSettingsStore{
		settings: &domain.Settings{
			TeamID:         "team-1",
			ResultsPerPage: 10,
			UpdatedAt:      time.Now().Add(-time.Hour),
		},
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resultsPerPage := 25
	req := driving.UpdateSettingsRequest{
		ResultsPerPage: &resultsPerPage,
	}

	settings, err := svc.Update(context.Background(), "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ResultsPerPage != 25 {
		t.Errorf("expected 25, got %d", settings.ResultsPerPage)
	}
}
