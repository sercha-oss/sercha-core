package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// mockCapabilityStore implements driven.CapabilityStore for testing
type mockCapabilityStore struct {
	getPreferencesErr  error
	savePreferencesErr error
	preferences        *domain.CapabilityPreferences
}

func (m *mockCapabilityStore) GetPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error) {
	if m.getPreferencesErr != nil {
		return nil, m.getPreferencesErr
	}
	if m.preferences != nil {
		return m.preferences, nil
	}
	// Return defaults if no preferences set
	return domain.DefaultCapabilityPreferences(teamID), nil
}

func (m *mockCapabilityStore) SavePreferences(ctx context.Context, prefs *domain.CapabilityPreferences) error {
	if m.savePreferencesErr != nil {
		return m.savePreferencesErr
	}
	m.preferences = prefs
	return nil
}

// mockCapabilitiesConfigProvider implements driven.ConfigProvider for capabilities service testing
type mockCapabilitiesConfigProvider struct {
	oauthCredentials map[domain.ProviderType]*driven.OAuthCredentials
	aiCredentials    map[domain.AIProvider]*driven.AICredentials
	baseURL          string
	capabilities     *driven.Capabilities
}

func newMockCapabilitiesConfigProvider() *mockCapabilitiesConfigProvider {
	return &mockCapabilitiesConfigProvider{
		oauthCredentials: make(map[domain.ProviderType]*driven.OAuthCredentials),
		aiCredentials:    make(map[domain.AIProvider]*driven.AICredentials),
		baseURL:          "http://localhost:3000",
		capabilities: &driven.Capabilities{
			OAuthProviders:     []domain.ProviderType{},
			EmbeddingProviders: []domain.AIProvider{domain.AIProviderOpenAI},
			LLMProviders:       []domain.AIProvider{domain.AIProviderOpenAI},
			Limits: driven.OperationalLimits{
				SyncMinInterval:   5,
				SyncMaxInterval:   1440,
				MaxWorkers:        10,
				MaxResultsPerPage: 100,
			},
		},
	}
}

func (m *mockCapabilitiesConfigProvider) GetOAuthCredentials(provider domain.ProviderType) *driven.OAuthCredentials {
	return m.oauthCredentials[provider]
}

func (m *mockCapabilitiesConfigProvider) GetAICredentials(provider domain.AIProvider) *driven.AICredentials {
	return m.aiCredentials[provider]
}

func (m *mockCapabilitiesConfigProvider) IsOAuthConfigured(provider domain.ProviderType) bool {
	return m.oauthCredentials[provider] != nil
}

func (m *mockCapabilitiesConfigProvider) IsAIConfigured(provider domain.AIProvider) bool {
	return m.aiCredentials[provider] != nil
}

func (m *mockCapabilitiesConfigProvider) GetCapabilities() *driven.Capabilities {
	if m.capabilities != nil {
		return m.capabilities
	}
	return &driven.Capabilities{
		OAuthProviders:     []domain.ProviderType{},
		EmbeddingProviders: []domain.AIProvider{},
		LLMProviders:       []domain.AIProvider{},
	}
}

func (m *mockCapabilitiesConfigProvider) GetBaseURL() string {
	return m.baseURL
}

// TestGetCapabilities tests the GetCapabilities method
func TestGetCapabilities(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	configProvider.oauthCredentials[domain.ProviderTypeGitHub] = &driven.OAuthCredentials{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	configProvider.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
		APIKey: "test-api-key",
	}
	configProvider.capabilities = &driven.Capabilities{
		OAuthProviders:     []domain.ProviderType{domain.ProviderTypeGitHub},
		EmbeddingProviders: []domain.AIProvider{domain.AIProviderOpenAI},
		LLMProviders:       []domain.AIProvider{domain.AIProviderOpenAI},
		Limits: driven.OperationalLimits{
			SyncMinInterval:   5,
			SyncMaxInterval:   1440,
			MaxWorkers:        10,
			MaxResultsPerPage: 100,
		},
	}

	store := &mockCapabilityStore{}
	svc := NewCapabilitiesService(configProvider, store)

	caps, err := svc.GetCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(caps.OAuthProviders) != 1 || caps.OAuthProviders[0] != domain.ProviderTypeGitHub {
		t.Errorf("expected GitHub provider, got %v", caps.OAuthProviders)
	}

	if len(caps.AIProviders.Embedding) != 1 || caps.AIProviders.Embedding[0] != domain.AIProviderOpenAI {
		t.Errorf("expected OpenAI embedding provider, got %v", caps.AIProviders.Embedding)
	}

	if caps.Limits.SyncMinInterval != 5 {
		t.Errorf("expected SyncMinInterval 5, got %d", caps.Limits.SyncMinInterval)
	}
}

// TestGetCapabilityPreferences_Success tests successful retrieval of preferences
func TestGetCapabilityPreferences_Success(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      true,
			EmbeddingIndexingEnabled: true,
			BM25SearchEnabled:        true,
			VectorSearchEnabled:      true,
			UpdatedAt:                time.Now(),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	prefs, err := svc.GetCapabilityPreferences(context.Background(), "team-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prefs.TeamID != "team-123" {
		t.Errorf("expected team-123, got %s", prefs.TeamID)
	}
	if !prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to be true")
	}
	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be true")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be true")
	}
}

// TestGetCapabilityPreferences_DefaultsWhenNotFound tests that defaults are returned when not found
func TestGetCapabilityPreferences_DefaultsWhenNotFound(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	// Store returns nil/defaults when no preferences exist
	store := &mockCapabilityStore{
		preferences: nil, // Will return defaults
	}
	svc := NewCapabilitiesService(configProvider, store)

	prefs, err := svc.GetCapabilityPreferences(context.Background(), "new-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prefs.TeamID != "new-team" {
		t.Errorf("expected new-team, got %s", prefs.TeamID)
	}
	// Verify defaults from domain.DefaultCapabilityPreferences
	if !prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to be true (default)")
	}
	if prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be false (default)")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be true (default)")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be true (default)")
	}
}

// TestGetCapabilityPreferences_StoreError tests error propagation from store
func TestGetCapabilityPreferences_StoreError(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	expectedErr := errors.New("database connection failed")
	store := &mockCapabilityStore{
		getPreferencesErr: expectedErr,
	}
	svc := NewCapabilitiesService(configProvider, store)

	prefs, err := svc.GetCapabilityPreferences(context.Background(), "team-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if prefs != nil {
		t.Errorf("expected nil preferences on error, got %v", prefs)
	}
	// Error should be wrapped
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to contain %q, got %q", expectedErr, err)
	}
}

// TestUpdateCapabilityPreferences_Success tests successful update of preferences
func TestUpdateCapabilityPreferences_Success(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      true,
			EmbeddingIndexingEnabled: false,
			BM25SearchEnabled:        true,
			VectorSearchEnabled:      false,
			UpdatedAt:                time.Now().Add(-time.Hour),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	// Enable embedding indexing
	enabled := true
	req := driving.UpdateCapabilityPreferencesRequest{
		EmbeddingIndexingEnabled: &enabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be true after update")
	}
	// Vector search should also be enabled when embedding is enabled
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be true when embedding is enabled")
	}
}

// TestUpdateCapabilityPreferences_PartialUpdate tests that only specified fields change
func TestUpdateCapabilityPreferences_PartialUpdate(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	originalPrefs := &domain.CapabilityPreferences{
		TeamID:                   "team-123",
		TextIndexingEnabled:      true,
		EmbeddingIndexingEnabled: true,
		BM25SearchEnabled:        true,
		VectorSearchEnabled:      true,
		UpdatedAt:                time.Now().Add(-time.Hour),
	}
	store := &mockCapabilityStore{
		preferences: originalPrefs,
	}
	svc := NewCapabilitiesService(configProvider, store)

	// Only disable BM25 search
	disabled := false
	req := driving.UpdateCapabilityPreferencesRequest{
		BM25SearchEnabled: &disabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only BM25SearchEnabled should change
	if prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be false after update")
	}
	// Other fields should remain unchanged
	if !prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to remain true")
	}
	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to remain true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to remain true")
	}
}

// TestUpdateCapabilityPreferences_AllFields tests updating all fields at once
func TestUpdateCapabilityPreferences_AllFields(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: domain.DefaultCapabilityPreferences("team-123"),
	}
	svc := NewCapabilitiesService(configProvider, store)

	// Update all fields
	textEnabled := false
	embeddingEnabled := true
	bm25Enabled := false
	vectorEnabled := true

	req := driving.UpdateCapabilityPreferencesRequest{
		TextIndexingEnabled:      &textEnabled,
		EmbeddingIndexingEnabled: &embeddingEnabled,
		BM25SearchEnabled:        &bm25Enabled,
		VectorSearchEnabled:      &vectorEnabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Note: DisableTextIndexing will set both TextIndexing and BM25Search to false
	if prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to be false")
	}
	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be true")
	}
	// BM25 search is disabled by DisableTextIndexing
	if prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be false")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be true")
	}
}

// TestUpdateCapabilityPreferences_StoreError tests error propagation from store on save
func TestUpdateCapabilityPreferences_StoreError(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences:        domain.DefaultCapabilityPreferences("team-123"),
		savePreferencesErr: errors.New("database write error"),
	}
	svc := NewCapabilitiesService(configProvider, store)

	enabled := true
	req := driving.UpdateCapabilityPreferencesRequest{
		EmbeddingIndexingEnabled: &enabled,
	}

	_, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error should mention "save capability preferences"
	if !errors.Is(err, store.savePreferencesErr) {
		t.Logf("Error message: %v", err)
	}
}

// TestUpdateCapabilityPreferences_DisableTextIndexing tests disabling text indexing disables BM25 search
func TestUpdateCapabilityPreferences_DisableTextIndexing(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      true,
			EmbeddingIndexingEnabled: false,
			BM25SearchEnabled:        true,
			VectorSearchEnabled:      false,
			UpdatedAt:                time.Now(),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	disabled := false
	req := driving.UpdateCapabilityPreferencesRequest{
		TextIndexingEnabled: &disabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to be false")
	}
	// BM25 search should also be disabled when text indexing is disabled
	if prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be false when text indexing is disabled")
	}
}

// TestUpdateCapabilityPreferences_DisableEmbeddingIndexing tests disabling embedding indexing disables vector search
func TestUpdateCapabilityPreferences_DisableEmbeddingIndexing(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      true,
			EmbeddingIndexingEnabled: true,
			BM25SearchEnabled:        true,
			VectorSearchEnabled:      true,
			UpdatedAt:                time.Now(),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	disabled := false
	req := driving.UpdateCapabilityPreferencesRequest{
		EmbeddingIndexingEnabled: &disabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be false")
	}
	// Vector search should also be disabled when embedding indexing is disabled
	if prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be false when embedding indexing is disabled")
	}
}

// TestUpdateCapabilityPreferences_GetPreferencesError tests error handling when getting existing preferences fails
func TestUpdateCapabilityPreferences_GetPreferencesError(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	// When GetPreferences fails, UpdateCapabilityPreferences should start with defaults
	store := &mockCapabilityStore{
		getPreferencesErr: errors.New("database read error"),
	}
	svc := NewCapabilitiesService(configProvider, store)

	enabled := true
	req := driving.UpdateCapabilityPreferencesRequest{
		EmbeddingIndexingEnabled: &enabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have started with defaults and then applied the update
	if prefs.TeamID != "team-123" {
		t.Errorf("expected team-123, got %s", prefs.TeamID)
	}
	// Embedding should be enabled per our request
	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be true")
	}
}

// TestUpdateCapabilityPreferences_EnableTextIndexing tests enabling text indexing enables BM25 search
func TestUpdateCapabilityPreferences_EnableTextIndexing(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      false,
			EmbeddingIndexingEnabled: false,
			BM25SearchEnabled:        false,
			VectorSearchEnabled:      false,
			UpdatedAt:                time.Now(),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	enabled := true
	req := driving.UpdateCapabilityPreferencesRequest{
		TextIndexingEnabled: &enabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to be true")
	}
	// BM25 search should also be enabled when text indexing is enabled
	if !prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to be true when text indexing is enabled")
	}
}

// TestUpdateCapabilityPreferences_EnableEmbeddingIndexing tests enabling embedding indexing enables vector search
func TestUpdateCapabilityPreferences_EnableEmbeddingIndexing(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      false,
			EmbeddingIndexingEnabled: false,
			BM25SearchEnabled:        false,
			VectorSearchEnabled:      false,
			UpdatedAt:                time.Now(),
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	enabled := true
	req := driving.UpdateCapabilityPreferencesRequest{
		EmbeddingIndexingEnabled: &enabled,
	}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to be true")
	}
	// Vector search should also be enabled when embedding indexing is enabled
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to be true when embedding indexing is enabled")
	}
}

// TestUpdateCapabilityPreferences_EmptyRequest tests that empty request preserves existing values
func TestUpdateCapabilityPreferences_EmptyRequest(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	originalUpdatedAt := time.Now().Add(-time.Hour)
	store := &mockCapabilityStore{
		preferences: &domain.CapabilityPreferences{
			TeamID:                   "team-123",
			TextIndexingEnabled:      true,
			EmbeddingIndexingEnabled: true,
			BM25SearchEnabled:        true,
			VectorSearchEnabled:      true,
			UpdatedAt:                originalUpdatedAt,
		},
	}
	svc := NewCapabilitiesService(configProvider, store)

	// Empty request - no fields specified
	req := driving.UpdateCapabilityPreferencesRequest{}

	prefs, err := svc.UpdateCapabilityPreferences(context.Background(), "team-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All values should remain unchanged
	if !prefs.TextIndexingEnabled {
		t.Error("expected TextIndexingEnabled to remain true")
	}
	if !prefs.EmbeddingIndexingEnabled {
		t.Error("expected EmbeddingIndexingEnabled to remain true")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("expected BM25SearchEnabled to remain true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("expected VectorSearchEnabled to remain true")
	}
}

// TestGetCapabilities_NoProviders tests GetCapabilities with no providers configured
func TestGetCapabilities_NoProviders(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	configProvider.capabilities = &driven.Capabilities{
		OAuthProviders:     []domain.ProviderType{},
		EmbeddingProviders: []domain.AIProvider{},
		LLMProviders:       []domain.AIProvider{},
		Limits: driven.OperationalLimits{
			SyncMinInterval:   5,
			SyncMaxInterval:   1440,
			MaxWorkers:        10,
			MaxResultsPerPage: 100,
		},
	}

	store := &mockCapabilityStore{}
	svc := NewCapabilitiesService(configProvider, store)

	caps, err := svc.GetCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(caps.OAuthProviders) != 0 {
		t.Errorf("expected no OAuth providers, got %v", caps.OAuthProviders)
	}

	if len(caps.AIProviders.Embedding) != 0 {
		t.Errorf("expected no embedding providers, got %v", caps.AIProviders.Embedding)
	}

	if len(caps.AIProviders.LLM) != 0 {
		t.Errorf("expected no LLM providers, got %v", caps.AIProviders.LLM)
	}

	// Features should reflect no backends available
	if caps.Features.TextIndexing {
		t.Error("expected TextIndexing to be false with no search engine")
	}
	if caps.Features.EmbeddingIndexing {
		t.Error("expected EmbeddingIndexing to be false with no embedding providers")
	}
	if caps.Features.BM25Search {
		t.Error("expected BM25Search to be false with no search engine")
	}
	if caps.Features.VectorSearch {
		t.Error("expected VectorSearch to be false with no embedding providers")
	}
}

// TestGetCapabilities_WithEmbeddingProviders tests feature flags with embedding providers
func TestGetCapabilities_WithEmbeddingProviders(t *testing.T) {
	configProvider := newMockCapabilitiesConfigProvider()
	configProvider.capabilities = &driven.Capabilities{
		OAuthProviders:        []domain.ProviderType{},
		EmbeddingProviders:    []domain.AIProvider{domain.AIProviderOpenAI},
		LLMProviders:          []domain.AIProvider{},
		SearchEngineAvailable: true,
		VectorStoreAvailable:  true,
		Limits: driven.OperationalLimits{
			SyncMinInterval:   5,
			SyncMaxInterval:   1440,
			MaxWorkers:        10,
			MaxResultsPerPage: 100,
		},
	}

	store := &mockCapabilityStore{}
	svc := NewCapabilitiesService(configProvider, store)

	caps, err := svc.GetCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Features should reflect all backends available
	if !caps.Features.TextIndexing {
		t.Error("expected TextIndexing to be true with search engine")
	}
	if !caps.Features.EmbeddingIndexing {
		t.Error("expected EmbeddingIndexing to be true with embedding providers + vector store")
	}
	if !caps.Features.BM25Search {
		t.Error("expected BM25Search to be true with search engine")
	}
	if !caps.Features.VectorSearch {
		t.Error("expected VectorSearch to be true with embedding providers + vector store")
	}
}
