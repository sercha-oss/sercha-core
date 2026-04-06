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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for local testing

// MockVespaDeployer is a mock implementation of driven.VespaDeployer
type MockVespaDeployer struct {
	mock.Mock
}

func (m *MockVespaDeployer) Deploy(ctx context.Context, endpoint string, embeddingDim *int, existingPkg *driven.AppPackage) (*domain.VespaDeployResult, error) {
	args := m.Called(ctx, endpoint, embeddingDim, existingPkg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VespaDeployResult), args.Error(1)
}

func (m *MockVespaDeployer) FetchAppPackage(ctx context.Context, endpoint string) (*driven.AppPackage, error) {
	args := m.Called(ctx, endpoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driven.AppPackage), args.Error(1)
}

func (m *MockVespaDeployer) GetSchemaInfo(ctx context.Context, endpoint string) (*driven.SchemaInfo, error) {
	args := m.Called(ctx, endpoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driven.SchemaInfo), args.Error(1)
}

func (m *MockVespaDeployer) HealthCheck(ctx context.Context, endpoint string) error {
	args := m.Called(ctx, endpoint)
	return args.Error(0)
}

func (m *MockVespaDeployer) GetMetrics(ctx context.Context, metricsEndpoint string) (*domain.VespaMetrics, error) {
	args := m.Called(ctx, metricsEndpoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VespaMetrics), args.Error(1)
}

// MockVespaConfigStore is a mock implementation of driven.VespaConfigStore
type MockVespaConfigStore struct {
	mock.Mock
}

func (m *MockVespaConfigStore) GetVespaConfig(ctx context.Context, teamID string) (*domain.VespaConfig, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VespaConfig), args.Error(1)
}

func (m *MockVespaConfigStore) SaveVespaConfig(ctx context.Context, config *domain.VespaConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// MockSettingsStore is a mock implementation of driven.SettingsStore
type MockSettingsStore struct {
	mock.Mock
}

func (m *MockSettingsStore) GetSettings(ctx context.Context, teamID string) (*domain.Settings, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Settings), args.Error(1)
}

func (m *MockSettingsStore) SaveSettings(ctx context.Context, settings *domain.Settings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

func (m *MockSettingsStore) GetAISettings(ctx context.Context, teamID string) (*domain.AISettings, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AISettings), args.Error(1)
}

func (m *MockSettingsStore) SaveAISettings(ctx context.Context, teamID string, settings *domain.AISettings) error {
	args := m.Called(ctx, teamID, settings)
	return args.Error(0)
}

// MockEmbeddingService is a mock implementation of driven.EmbeddingService
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float32), args.Error(1)
}

func (m *MockEmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockEmbeddingService) Dimensions() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockEmbeddingService) Model() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEmbeddingService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockEmbeddingService) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockSearchEngine is a mock implementation of driven.SearchEngine
type MockSearchEngine struct {
	mock.Mock
}

func (m *MockSearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	args := m.Called(ctx, chunks)
	return args.Error(0)
}

func (m *MockSearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	args := m.Called(ctx, query, queryEmbedding, opts)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.RankedChunk), args.Int(1), args.Error(2)
}

func (m *MockSearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	args := m.Called(ctx, chunkIDs)
	return args.Error(0)
}

func (m *MockSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	args := m.Called(ctx, documentID)
	return args.Error(0)
}

func (m *MockSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	args := m.Called(ctx, sourceID)
	return args.Error(0)
}

func (m *MockSearchEngine) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSearchEngine) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// MockConfigProvider is a mock implementation of driven.ConfigProvider
type MockConfigProvider struct {
	mock.Mock
}

func (m *MockConfigProvider) GetOAuthCredentials(provider domain.ProviderType) *driven.OAuthCredentials {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*driven.OAuthCredentials)
}

func (m *MockConfigProvider) GetAICredentials(provider domain.AIProvider) *driven.AICredentials {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*driven.AICredentials)
}

func (m *MockConfigProvider) IsOAuthConfigured(provider domain.ProviderType) bool {
	args := m.Called(provider)
	return args.Bool(0)
}

func (m *MockConfigProvider) IsAIConfigured(provider domain.AIProvider) bool {
	args := m.Called(provider)
	return args.Bool(0)
}

func (m *MockConfigProvider) GetCapabilities() *driven.Capabilities {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*driven.Capabilities)
}

func (m *MockConfigProvider) GetBaseURL() string {
	args := m.Called()
	return args.String(0)
}

// Test Helpers

func setupVespaAdminTest(t *testing.T) (*vespaAdminService, *MockVespaDeployer, *MockVespaConfigStore, *MockSettingsStore, *MockSearchEngine, *MockConfigProvider, *runtime.Services) {
	deployer := new(MockVespaDeployer)
	configStore := new(MockVespaConfigStore)
	settingsStore := new(MockSettingsStore)
	searchEngine := new(MockSearchEngine)
	configProvider := new(MockConfigProvider)
	runtimeConfig := domain.NewRuntimeConfig("redis")
	services := runtime.NewServices(runtimeConfig)
	teamID := "test-team-123"

	// Default: no embedding providers configured
	configProvider.On("GetCapabilities").Return(&driven.Capabilities{
		EmbeddingProviders: []domain.AIProvider{},
	}).Maybe()

	svc := &vespaAdminService{
		deployer:        deployer,
		configStore:     configStore,
		settingsStore:   settingsStore,
		searchEngine:    searchEngine,
		configProvider:  configProvider,
		services:        services,
		teamID:          teamID,
		defaultEndpoint: "http://localhost:19071",
	}

	return svc, deployer, configStore, settingsStore, searchEngine, configProvider, services
}

// TestNewVespaAdminService tests the constructor
func TestNewVespaAdminService(t *testing.T) {
	deployer := new(MockVespaDeployer)
	configStore := new(MockVespaConfigStore)
	settingsStore := new(MockSettingsStore)
	searchEngine := new(MockSearchEngine)
	configProvider := new(MockConfigProvider)
	runtimeConfig := domain.NewRuntimeConfig("redis")
	services := runtime.NewServices(runtimeConfig)
	teamID := "test-team-123"
	defaultEndpoint := "http://localhost:19071"

	svc := NewVespaAdminService(deployer, configStore, settingsStore, searchEngine, configProvider, services, teamID, defaultEndpoint)

	require.NotNil(t, svc)
	assert.Implements(t, (*driving.VespaAdminService)(nil), svc)
}

// TestConnect_DevMode_BM25Only tests connecting in dev mode without embeddings
func TestConnect_DevMode_BM25Only(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Deploy succeeds (BM25-only mode, no embeddings)
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://localhost:8080", (*int)(nil), (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, "http://localhost:8080", status.Endpoint)
	assert.True(t, status.DevMode)
	assert.Equal(t, domain.VespacSchemaModeBM25, status.SchemaMode)
	assert.False(t, status.EmbeddingsEnabled)
	assert.Equal(t, 0, status.EmbeddingDim)
	assert.True(t, status.Healthy)
	assert.False(t, status.ReindexRequired)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_DevMode_WithEmbeddings tests connecting in dev mode with embeddings
func TestConnect_DevMode_WithEmbeddings(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, services := setupVespaAdminTest(t)

	// Setup embedding service
	mockEmbed := new(MockEmbeddingService)
	mockEmbed.On("Dimensions").Return(1536)
	services.SetEmbeddingService(mockEmbed)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// AI settings with provider info
	aiSettings := &domain.AISettings{
		TeamID: "test-team-123",
		Embedding: domain.EmbeddingSettings{
			Provider: domain.AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
	}
	settingsStore.On("GetAISettings", ctx, "test-team-123").Return(aiSettings, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Deploy succeeds with hybrid mode
	embeddingDim := 1536
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeHybrid,
		EmbeddingDim:  embeddingDim,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://localhost:8080", &embeddingDim, (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, "http://localhost:8080", status.Endpoint)
	assert.True(t, status.DevMode)
	assert.Equal(t, domain.VespacSchemaModeHybrid, status.SchemaMode)
	assert.True(t, status.EmbeddingsEnabled)
	assert.Equal(t, 1536, status.EmbeddingDim)
	assert.Equal(t, domain.AIProviderOpenAI, status.EmbeddingProvider)
	assert.True(t, status.Healthy)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	mockEmbed.AssertExpectations(t)
}

// TestConnect_ProductionMode_Success tests connecting in production mode
func TestConnect_ProductionMode_Success(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://prod-vespa:8080",
		DevMode:  false,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://prod-vespa:8080").Return(nil)

	// Fetch existing app package
	existingPkg := &driven.AppPackage{
		ServicesXML: "<services/>",
		Schemas:     map[string]string{},
		ClusterInfo: &domain.VespaClusterInfo{
			OurSchemaDeployed: false,
		},
	}
	deployer.On("FetchAppPackage", ctx, "http://prod-vespa:8080").Return(existingPkg, nil)

	// Deploy succeeds with existing package
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://prod-vespa:8080", (*int)(nil), existingPkg).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, "http://prod-vespa:8080", status.Endpoint)
	assert.False(t, status.DevMode)
	assert.NotNil(t, status.ClusterInfo)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_ProductionMode_NoExistingApp tests production mode when no app is deployed
func TestConnect_ProductionMode_NoExistingApp(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://prod-vespa:8080",
		DevMode:  false,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://prod-vespa:8080").Return(nil)

	// Fetch returns nil (no existing app)
	deployer.On("FetchAppPackage", ctx, "http://prod-vespa:8080").Return(nil, nil)

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "no existing Vespa application found")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_HealthCheckFails tests connect when health check fails
func TestConnect_HealthCheckFails(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check fails
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(errors.New("connection refused"))

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "vespa health check failed")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_CannotDowngradeFromHybrid tests that we can't downgrade from hybrid to BM25
func TestConnect_CannotDowngradeFromHybrid(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Existing config has hybrid mode
	existingConfig := &domain.VespaConfig{
		TeamID:        "test-team-123",
		Endpoint:      "http://localhost:8080",
		Connected:     true,
		SchemaMode:    domain.VespacSchemaModeHybrid,
		EmbeddingDim:  1536,
		SchemaVersion: "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// No embedding service (trying to downgrade)
	// services.EmbeddingService() will return nil

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "cannot downgrade from hybrid to BM25-only")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_CannotChangeDimension tests that we can't change embedding dimension
func TestConnect_CannotChangeDimension(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, services := setupVespaAdminTest(t)

	// Setup embedding service with different dimension
	mockEmbed := new(MockEmbeddingService)
	mockEmbed.On("Dimensions").Return(768) // Different from existing 1536
	services.SetEmbeddingService(mockEmbed)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Existing config has hybrid mode with 1536 dimensions
	existingConfig := &domain.VespaConfig{
		TeamID:        "test-team-123",
		Endpoint:      "http://localhost:8080",
		Connected:     true,
		SchemaMode:    domain.VespacSchemaModeHybrid,
		EmbeddingDim:  1536,
		SchemaVersion: "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// AI settings
	aiSettings := &domain.AISettings{
		TeamID: "test-team-123",
		Embedding: domain.EmbeddingSettings{
			Provider: domain.AIProviderOpenAI,
		},
	}
	settingsStore.On("GetAISettings", ctx, "test-team-123").Return(aiSettings, nil)

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "cannot change embedding dimension from 1536 to 768")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	mockEmbed.AssertExpectations(t)
}

// TestConnect_DeployFails tests when deployment fails
func TestConnect_DeployFails(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Deploy fails
	deployer.On("Deploy", ctx, "http://localhost:8080", (*int)(nil), (*driven.AppPackage)(nil)).Return(nil, errors.New("deployment failed"))

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "vespa schema deployment failed")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_SaveConfigFails tests when config save fails
func TestConnect_SaveConfigFails(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Deploy succeeds
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://localhost:8080", (*int)(nil), (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save fails
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(errors.New("database error"))

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to save vespa config")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_DefaultEndpoint tests using default endpoint when not provided
func TestConnect_DefaultEndpoint(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "", // Empty - should use default
		DevMode:  true,
	}

	// Config store returns error (no existing config)
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	// Health check succeeds with default endpoint
	deployer.On("HealthCheck", ctx, "http://vespa:19071").Return(nil)

	// Deploy succeeds
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://vespa:19071", (*int)(nil), (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "http://vespa:19071", status.Endpoint)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_UseStoredEndpoint tests using stored endpoint from config
func TestConnect_UseStoredEndpoint(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "", // Empty - should use stored endpoint
		DevMode:  true,
	}

	// Existing config has stored endpoint
	existingConfig := &domain.VespaConfig{
		TeamID:    "test-team-123",
		Endpoint:  "http://stored-vespa:9999",
		Connected: false,
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds with stored endpoint
	deployer.On("HealthCheck", ctx, "http://stored-vespa:9999").Return(nil)

	// Deploy succeeds
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://stored-vespa:9999", (*int)(nil), (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "http://stored-vespa:9999", status.Endpoint)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_UpgradeFromBM25ToHybrid tests upgrading schema from BM25 to hybrid
func TestConnect_UpgradeFromBM25ToHybrid(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, services := setupVespaAdminTest(t)

	// Setup embedding service
	mockEmbed := new(MockEmbeddingService)
	mockEmbed.On("Dimensions").Return(1536)
	services.SetEmbeddingService(mockEmbed)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	// Existing config has BM25 mode
	existingConfig := &domain.VespaConfig{
		TeamID:        "test-team-123",
		Endpoint:      "http://localhost:8080",
		Connected:     true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// AI settings
	aiSettings := &domain.AISettings{
		TeamID: "test-team-123",
		Embedding: domain.EmbeddingSettings{
			Provider: domain.AIProviderOpenAI,
		},
	}
	settingsStore.On("GetAISettings", ctx, "test-team-123").Return(aiSettings, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Deploy succeeds with upgrade
	embeddingDim := 1536
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeHybrid,
		EmbeddingDim:  embeddingDim,
		SchemaVersion: "v2.0.0",
		Upgraded:      true,
	}
	deployer.On("Deploy", ctx, "http://localhost:8080", &embeddingDim, (*driven.AppPackage)(nil)).Return(deployResult, nil)

	// Config save succeeds
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, domain.VespacSchemaModeHybrid, status.SchemaMode)
	assert.True(t, status.EmbeddingsEnabled)
	assert.True(t, status.ReindexRequired)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	mockEmbed.AssertExpectations(t)
}

// TestStatus_Connected tests status when Vespa is connected and healthy
func TestStatus_Connected(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, searchEngine, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:            "test-team-123",
		Endpoint:          "http://localhost:8080",
		Connected:         true,
		DevMode:           true,
		SchemaMode:        domain.VespacSchemaModeBM25,
		SchemaVersion:     "v1.0.0",
		EmbeddingDim:      0,
		EmbeddingProvider: "",
		ConnectedAt:       time.Now(),
		UpdatedAt:         time.Now(),
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Count returns indexed chunks
	searchEngine.On("Count", ctx).Return(int64(42), nil)

	status, err := svc.Status(ctx)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, "http://localhost:8080", status.Endpoint)
	assert.True(t, status.Healthy)
	assert.Equal(t, domain.VespacSchemaModeBM25, status.SchemaMode)
	assert.False(t, status.EmbeddingsEnabled)
	assert.Equal(t, int64(42), status.IndexedChunks)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	searchEngine.AssertExpectations(t)
}

// TestStatus_NotConfigured tests status when Vespa is not configured
func TestStatus_NotConfigured(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	// Config not found
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	status, err := svc.Status(ctx)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.Connected)
	assert.Equal(t, "http://localhost:19071", status.Endpoint)
	assert.False(t, status.Healthy)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestStatus_ConnectedButUnhealthy tests status when configured but Vespa is down
func TestStatus_ConnectedButUnhealthy(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:        "test-team-123",
		Endpoint:      "http://localhost:8080",
		Connected:     true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check fails
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(errors.New("connection refused"))

	status, err := svc.Status(ctx)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.False(t, status.Healthy)
	assert.Equal(t, int64(0), status.IndexedChunks) // Not counted when unhealthy

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestStatus_CanUpgrade tests status can upgrade flag
func TestStatus_CanUpgrade(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, searchEngine, configProvider, _ := setupVespaAdminTest(t)

	// Configure capabilities with embedding providers (overrides default empty from setup)
	configProvider.ExpectedCalls = nil // Clear default mock
	configProvider.On("GetCapabilities").Return(&driven.Capabilities{
		EmbeddingProviders: []domain.AIProvider{domain.AIProviderOpenAI},
	})

	// Config is BM25-only
	existingConfig := &domain.VespaConfig{
		TeamID:        "test-team-123",
		Endpoint:      "http://localhost:8080",
		Connected:     true,
		SchemaMode:    domain.VespacSchemaModeBM25,
		SchemaVersion: "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Count returns indexed chunks
	searchEngine.On("Count", ctx).Return(int64(100), nil)

	status, err := svc.Status(ctx)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.CanUpgrade) // Should be true since embedding providers are configured

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	searchEngine.AssertExpectations(t)
}

// TestStatus_HybridMode tests status with hybrid mode enabled
func TestStatus_HybridMode(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, searchEngine, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:            "test-team-123",
		Endpoint:          "http://localhost:8080",
		Connected:         true,
		SchemaMode:        domain.VespacSchemaModeHybrid,
		EmbeddingDim:      1536,
		EmbeddingProvider: domain.AIProviderOpenAI,
		SchemaVersion:     "v1.0.0",
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	// Count returns indexed chunks
	searchEngine.On("Count", ctx).Return(int64(500), nil)

	status, err := svc.Status(ctx)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.True(t, status.Healthy)
	assert.Equal(t, domain.VespacSchemaModeHybrid, status.SchemaMode)
	assert.True(t, status.EmbeddingsEnabled)
	assert.Equal(t, 1536, status.EmbeddingDim)
	assert.Equal(t, domain.AIProviderOpenAI, status.EmbeddingProvider)
	assert.False(t, status.CanUpgrade) // Can't upgrade from hybrid
	assert.Equal(t, int64(500), status.IndexedChunks)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	searchEngine.AssertExpectations(t)
}

// TestHealthCheck_Success tests successful health check
func TestHealthCheck_Success(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, searchEngine, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:    "test-team-123",
		Endpoint:  "http://localhost:8080",
		Connected: true,
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check succeeds
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)
	searchEngine.On("HealthCheck", ctx).Return(nil)

	err := svc.HealthCheck(ctx)

	assert.NoError(t, err)

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	searchEngine.AssertExpectations(t)
}

// TestHealthCheck_NotConfigured tests health check when not configured
func TestHealthCheck_NotConfigured(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	// Config not found
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))

	err := svc.HealthCheck(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vespa not configured")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestHealthCheck_NotConnected tests health check when not connected
func TestHealthCheck_NotConnected(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:    "test-team-123",
		Endpoint:  "http://localhost:8080",
		Connected: false,
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	err := svc.HealthCheck(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vespa not connected")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestHealthCheck_Fails tests health check when Vespa is down
func TestHealthCheck_Fails(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	existingConfig := &domain.VespaConfig{
		TeamID:    "test-team-123",
		Endpoint:  "http://localhost:8080",
		Connected: true,
	}
	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(existingConfig, nil)

	// Health check fails
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(errors.New("connection refused"))

	err := svc.HealthCheck(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}

// TestConnect_WithEmbeddingsButNoAISettings tests connecting with embeddings when AI settings fail to load
func TestConnect_WithEmbeddingsButNoAISettings(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, services := setupVespaAdminTest(t)

	// Setup embedding service
	mockEmbed := new(MockEmbeddingService)
	mockEmbed.On("Dimensions").Return(1536)
	services.SetEmbeddingService(mockEmbed)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	}

	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))
	settingsStore.On("GetAISettings", ctx, "test-team-123").Return(nil, errors.New("not found"))
	deployer.On("HealthCheck", ctx, "http://localhost:8080").Return(nil)

	embeddingDim := 1536
	deployResult := &domain.VespaDeployResult{
		Success:       true,
		SchemaMode:    domain.VespacSchemaModeHybrid,
		EmbeddingDim:  embeddingDim,
		SchemaVersion: "v1.0.0",
		Upgraded:      false,
	}
	deployer.On("Deploy", ctx, "http://localhost:8080", &embeddingDim, (*driven.AppPackage)(nil)).Return(deployResult, nil)
	configStore.On("SaveVespaConfig", ctx, mock.AnythingOfType("*domain.VespaConfig")).Return(nil)

	status, err := svc.Connect(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, domain.VespacSchemaModeHybrid, status.SchemaMode)
	assert.Equal(t, domain.AIProvider(""), status.EmbeddingProvider) // Empty provider when settings not found

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
	mockEmbed.AssertExpectations(t)
}

// TestConnect_ProductionMode_FetchFails tests production mode when fetch fails
func TestConnect_ProductionMode_FetchFails(t *testing.T) {
	ctx := context.Background()
	svc, deployer, configStore, settingsStore, _, _, _ := setupVespaAdminTest(t)

	req := driving.ConnectVespaRequest{
		Endpoint: "http://prod-vespa:8080",
		DevMode:  false,
	}

	configStore.On("GetVespaConfig", ctx, "test-team-123").Return(nil, errors.New("not found"))
	deployer.On("HealthCheck", ctx, "http://prod-vespa:8080").Return(nil)
	deployer.On("FetchAppPackage", ctx, "http://prod-vespa:8080").Return(nil, errors.New("fetch error"))

	status, err := svc.Connect(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to fetch existing app package")

	deployer.AssertExpectations(t)
	configStore.AssertExpectations(t)
	settingsStore.AssertExpectations(t)
}
