package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
	"github.com/sercha-oss/sercha-core/internal/runtime"
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
	// Note: SemanticSearchEnabled is now managed via CapabilityPreferences
	searchMode := domain.SearchModeHybrid
	resultsPerPage := 30
	syncInterval := 120
	syncEnabled := false
	req := driving.UpdateSettingsRequest{
		DefaultSearchMode:   &searchMode,
		ResultsPerPage:      &resultsPerPage,
		SyncIntervalMinutes: &syncInterval,
		SyncEnabled:         &syncEnabled,
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

func TestSettingsService_GetAIProviders(t *testing.T) {
	store := &mockSettingsStore{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resp, err := svc.GetAIProviders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response to be returned")
	}

	// Verify embedding providers
	if len(resp.Embedding) == 0 {
		t.Error("expected at least one embedding provider")
	}

	// Verify LLM providers
	if len(resp.LLM) == 0 {
		t.Error("expected at least one LLM provider")
	}

	// Verify OpenAI is in both
	foundOpenAIEmbedding := false
	for _, p := range resp.Embedding {
		if p.ID == string(domain.AIProviderOpenAI) {
			foundOpenAIEmbedding = true
			if p.Name != "OpenAI" {
				t.Errorf("expected name 'OpenAI', got %s", p.Name)
			}
			if !p.RequiresAPIKey {
				t.Error("expected OpenAI to require API key")
			}
			if p.RequiresBaseURL {
				t.Error("expected OpenAI to not require base URL")
			}
			if len(p.Models) == 0 {
				t.Error("expected OpenAI to have models")
			}
			if p.APIKeyURL == "" {
				t.Error("expected OpenAI to have API key URL")
			}
		}
	}
	if !foundOpenAIEmbedding {
		t.Error("expected OpenAI in embedding providers")
	}

	foundOpenAILLM := false
	for _, p := range resp.LLM {
		if p.ID == string(domain.AIProviderOpenAI) {
			foundOpenAILLM = true
			if len(p.Models) == 0 {
				t.Error("expected OpenAI LLM to have models")
			}
		}
	}
	if !foundOpenAILLM {
		t.Error("expected OpenAI in LLM providers")
	}
}

func TestSettingsService_GetAIProviders_IncludesAllExpectedProviders(t *testing.T) {
	store := &mockSettingsStore{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resp, err := svc.GetAIProviders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected embedding providers: OpenAI, Ollama, Cohere, Voyage
	expectedEmbedding := map[string]bool{
		string(domain.AIProviderOpenAI): false,
		string(domain.AIProviderOllama): false,
		string(domain.AIProviderCohere): false,
		string(domain.AIProviderVoyage): false,
	}

	for _, p := range resp.Embedding {
		if _, ok := expectedEmbedding[p.ID]; ok {
			expectedEmbedding[p.ID] = true
		}
	}

	for provider, found := range expectedEmbedding {
		if !found {
			t.Errorf("expected embedding provider %s not found", provider)
		}
	}

	// Expected LLM providers: OpenAI, Anthropic, Ollama
	expectedLLM := map[string]bool{
		string(domain.AIProviderOpenAI):    false,
		string(domain.AIProviderAnthropic): false,
		string(domain.AIProviderOllama):    false,
	}

	for _, p := range resp.LLM {
		if _, ok := expectedLLM[p.ID]; ok {
			expectedLLM[p.ID] = true
		}
	}

	for provider, found := range expectedLLM {
		if !found {
			t.Errorf("expected LLM provider %s not found", provider)
		}
	}
}

func TestSettingsService_GetAIProviders_OllamaConfiguration(t *testing.T) {
	store := &mockSettingsStore{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resp, err := svc.GetAIProviders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find Ollama in embedding providers
	for _, p := range resp.Embedding {
		if p.ID == string(domain.AIProviderOllama) {
			if p.RequiresAPIKey {
				t.Error("expected Ollama to not require API key")
			}
			if !p.RequiresBaseURL {
				t.Error("expected Ollama to require base URL")
			}
			if p.APIKeyURL != "" {
				t.Error("expected Ollama to not have API key URL")
			}
		}
	}

	// Find Ollama in LLM providers
	for _, p := range resp.LLM {
		if p.ID == string(domain.AIProviderOllama) {
			if p.RequiresAPIKey {
				t.Error("expected Ollama to not require API key")
			}
			if !p.RequiresBaseURL {
				t.Error("expected Ollama to require base URL")
			}
		}
	}
}

func TestSettingsService_GetAIProviders_ModelsHaveMetadata(t *testing.T) {
	store := &mockSettingsStore{}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	resp, err := svc.GetAIProviders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that embedding models have dimensions
	for _, p := range resp.Embedding {
		for _, m := range p.Models {
			if m.ID == "" {
				t.Errorf("provider %s has model with empty ID", p.ID)
			}
			if m.Name == "" {
				t.Errorf("provider %s has model with empty name", p.ID)
			}
			if m.Dimensions == 0 {
				t.Errorf("provider %s model %s has zero dimensions", p.ID, m.ID)
			}
		}
	}

	// Check that LLM models have basic metadata
	for _, p := range resp.LLM {
		for _, m := range p.Models {
			if m.ID == "" {
				t.Errorf("provider %s has model with empty ID", p.ID)
			}
			if m.Name == "" {
				t.Errorf("provider %s has model with empty name", p.ID)
			}
		}
	}
}

// TestSettingsService_UpdateWithSyncExclusions validates acceptance criteria:
// - Settings update handles `SyncExclusions` field
func TestSettingsService_UpdateWithSyncExclusions(t *testing.T) {
	tests := []struct {
		name            string
		initialSettings *domain.Settings
		updateRequest   driving.UpdateSettingsRequest
		validate        func(*testing.T, *domain.Settings)
	}{
		{
			name:            "update sync exclusions with enabled and custom patterns",
			initialSettings: domain.DefaultSettings("team-1"),
			updateRequest: driving.UpdateSettingsRequest{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns:  []string{".git/", "node_modules/", "*.log"},
					DisabledPatterns: []string{"build/"},
					CustomPatterns:   []string{"*.secret", "private/"},
				},
			},
			validate: func(t *testing.T, s *domain.Settings) {
				if s.SyncExclusions == nil {
					t.Fatal("expected SyncExclusions to be set")
				}
				if len(s.SyncExclusions.EnabledPatterns) != 3 {
					t.Errorf("expected 3 enabled patterns, got %d", len(s.SyncExclusions.EnabledPatterns))
				}
				if len(s.SyncExclusions.CustomPatterns) != 2 {
					t.Errorf("expected 2 custom patterns, got %d", len(s.SyncExclusions.CustomPatterns))
				}
				if len(s.SyncExclusions.DisabledPatterns) != 1 {
					t.Errorf("expected 1 disabled pattern, got %d", len(s.SyncExclusions.DisabledPatterns))
				}
			},
		},
		{
			name: "update sync exclusions to empty",
			initialSettings: func() *domain.Settings {
				s := domain.DefaultSettings("team-1")
				s.SyncExclusions = &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/"},
					CustomPatterns:  []string{"*.secret"},
				}
				return s
			}(),
			updateRequest: driving.UpdateSettingsRequest{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns:  []string{},
					DisabledPatterns: []string{},
					CustomPatterns:   []string{},
				},
			},
			validate: func(t *testing.T, s *domain.Settings) {
				if s.SyncExclusions == nil {
					t.Fatal("expected SyncExclusions to be set")
				}
				if len(s.SyncExclusions.EnabledPatterns) != 0 {
					t.Errorf("expected 0 enabled patterns, got %d", len(s.SyncExclusions.EnabledPatterns))
				}
				if len(s.SyncExclusions.CustomPatterns) != 0 {
					t.Errorf("expected 0 custom patterns, got %d", len(s.SyncExclusions.CustomPatterns))
				}
			},
		},
		{
			name:            "update only custom patterns",
			initialSettings: domain.DefaultSettings("team-1"),
			updateRequest: driving.UpdateSettingsRequest{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns:  []string{".git/"},
					DisabledPatterns: []string{},
					CustomPatterns:   []string{"my-pattern/", "*.custom"},
				},
			},
			validate: func(t *testing.T, s *domain.Settings) {
				if s.SyncExclusions == nil {
					t.Fatal("expected SyncExclusions to be set")
				}
				if len(s.SyncExclusions.CustomPatterns) != 2 {
					t.Errorf("expected 2 custom patterns, got %d", len(s.SyncExclusions.CustomPatterns))
				}
				hasCustomPattern := false
				for _, p := range s.SyncExclusions.CustomPatterns {
					if p == "my-pattern/" {
						hasCustomPattern = true
						break
					}
				}
				if !hasCustomPattern {
					t.Error("expected custom pattern 'my-pattern/' to be present")
				}
			},
		},
		{
			name: "nil sync exclusions in request - no update",
			initialSettings: func() *domain.Settings {
				s := domain.DefaultSettings("team-1")
				s.SyncExclusions = &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/"},
					CustomPatterns:  []string{"*.secret"},
				}
				return s
			}(),
			updateRequest: driving.UpdateSettingsRequest{
				// SyncExclusions is nil, should not modify existing
			},
			validate: func(t *testing.T, s *domain.Settings) {
				if s.SyncExclusions == nil {
					t.Fatal("expected SyncExclusions to remain set")
				}
				if len(s.SyncExclusions.EnabledPatterns) != 1 {
					t.Errorf("expected original enabled patterns to be preserved, got %d", len(s.SyncExclusions.EnabledPatterns))
				}
				if len(s.SyncExclusions.CustomPatterns) != 1 {
					t.Errorf("expected original custom patterns to be preserved, got %d", len(s.SyncExclusions.CustomPatterns))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockSettingsStore{
				settings: tt.initialSettings,
			}
			config := domain.NewRuntimeConfig("postgres")
			services := runtime.NewServices(config)
			configProvider := newMockConfigProvider()
			svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

			result, err := svc.Update(context.Background(), "user-1", tt.updateRequest)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result to be returned")
			}

			// Validate the result
			tt.validate(t, result)

			// Verify settings were saved
			if store.settings == nil {
				t.Fatal("expected settings to be saved")
			}
			tt.validate(t, store.settings)

			// Verify UpdatedBy is set
			if store.settings.UpdatedBy != "user-1" {
				t.Errorf("expected UpdatedBy to be 'user-1', got %s", store.settings.UpdatedBy)
			}

			// Verify UpdatedAt is set
			if store.settings.UpdatedAt.IsZero() {
				t.Error("expected UpdatedAt to be set")
			}
		})
	}
}

// TestSettingsService_UpdateMultipleFieldsWithSyncExclusions validates that
// multiple settings fields can be updated together including sync exclusions
func TestSettingsService_UpdateMultipleFieldsWithSyncExclusions(t *testing.T) {
	store := &mockSettingsStore{
		settings: domain.DefaultSettings("team-1"),
	}
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	configProvider := newMockConfigProvider()
	svc := NewSettingsService(store, &mockAIFactory{}, configProvider, services, "team-1")

	defaultMode := domain.SearchModeTextOnly
	resultsPerPage := 25
	syncInterval := 45
	syncEnabled := false

	req := driving.UpdateSettingsRequest{
		DefaultSearchMode:   &defaultMode,
		ResultsPerPage:      &resultsPerPage,
		SyncIntervalMinutes: &syncInterval,
		SyncEnabled:         &syncEnabled,
		SyncExclusions: &domain.SyncExclusionSettings{
			EnabledPatterns: []string{".git/"},
			CustomPatterns:  []string{"*.private"},
		},
	}

	result, err := svc.Update(context.Background(), "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields were updated
	if result.DefaultSearchMode != domain.SearchModeTextOnly {
		t.Errorf("expected DefaultSearchMode text, got %s", result.DefaultSearchMode)
	}
	if result.ResultsPerPage != 25 {
		t.Errorf("expected ResultsPerPage 25, got %d", result.ResultsPerPage)
	}
	if result.SyncIntervalMinutes != 45 {
		t.Errorf("expected SyncIntervalMinutes 45, got %d", result.SyncIntervalMinutes)
	}
	if result.SyncEnabled {
		t.Error("expected SyncEnabled to be false")
	}
	if result.SyncExclusions == nil {
		t.Fatal("expected SyncExclusions to be set")
	}
	if len(result.SyncExclusions.EnabledPatterns) != 1 {
		t.Errorf("expected 1 enabled pattern, got %d", len(result.SyncExclusions.EnabledPatterns))
	}
	if len(result.SyncExclusions.CustomPatterns) != 1 {
		t.Errorf("expected 1 custom pattern, got %d", len(result.SyncExclusions.CustomPatterns))
	}

	// Verify saved settings match
	if store.settings.DefaultSearchMode != result.DefaultSearchMode {
		t.Error("saved settings don't match result")
	}
	if store.settings.SyncExclusions == nil {
		t.Error("saved settings should have SyncExclusions")
	}
}
