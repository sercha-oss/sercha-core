package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Mock services for testing

type mockAuthService struct {
	authenticateFn  func(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error)
	validateTokenFn func(ctx context.Context, token string) (*domain.AuthContext, error)
	refreshTokenFn  func(ctx context.Context, req domain.RefreshRequest) (*domain.LoginResponse, error)
	logoutFn        func(ctx context.Context, token string) error
}

func (m *mockAuthService) Authenticate(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
	if m.authenticateFn != nil {
		return m.authenticateFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (*domain.AuthContext, error) {
	if m.validateTokenFn != nil {
		return m.validateTokenFn(ctx, token)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) RefreshToken(ctx context.Context, req domain.RefreshRequest) (*domain.LoginResponse, error) {
	if m.refreshTokenFn != nil {
		return m.refreshTokenFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) Logout(ctx context.Context, token string) error {
	if m.logoutFn != nil {
		return m.logoutFn(ctx, token)
	}
	return nil
}

func (m *mockAuthService) LogoutAll(ctx context.Context, userID string) error {
	return nil
}

func (m *mockAuthService) ChangePassword(ctx context.Context, userID string, req domain.ChangePasswordRequest) error {
	return nil
}

type mockUserService struct {
	setupFn  func(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error)
	createFn func(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error)
	getFn    func(ctx context.Context, id string) (*domain.User, error)
	listFn   func(ctx context.Context) ([]*domain.User, error)
	deleteFn func(ctx context.Context, id string) error
}

func (m *mockUserService) Setup(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
	if m.setupFn != nil {
		return m.setupFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) Create(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) Get(ctx context.Context, id string) (*domain.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserService) List(ctx context.Context) ([]*domain.User, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) Update(ctx context.Context, id string, req driving.UpdateUserRequest) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockUserService) SetPassword(ctx context.Context, id string, password string) error {
	return nil
}

type mockSearchService struct {
	searchFn func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error)
}

func (m *mockSearchService) Search(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, opts)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSearchService) SearchBySource(ctx context.Context, sourceID string, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSearchService) Suggest(ctx context.Context, prefix string, limit int) ([]domain.SearchSuggestion, error) {
	return nil, errors.New("not implemented")
}

type mockSourceService struct {
	createFn          func(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error)
	getFn             func(ctx context.Context, id string) (*domain.Source, error)
	listWithSummaryFn func(ctx context.Context) ([]*domain.SourceSummary, error)
	deleteFn          func(ctx context.Context, id string) error
}

func (m *mockSourceService) Create(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error) {
	if m.createFn != nil {
		return m.createFn(ctx, creatorID, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSourceService) Get(ctx context.Context, id string) (*domain.Source, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSourceService) List(ctx context.Context) ([]*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSourceService) ListWithSummary(ctx context.Context) ([]*domain.SourceSummary, error) {
	if m.listWithSummaryFn != nil {
		return m.listWithSummaryFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSourceService) Update(ctx context.Context, id string, req driving.UpdateSourceRequest) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSourceService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockSourceService) Enable(ctx context.Context, id string) error {
	return nil
}

func (m *mockSourceService) Disable(ctx context.Context, id string) error {
	return nil
}

func (m *mockSourceService) UpdateContainers(ctx context.Context, id string, containers []domain.Container) error {
	return nil
}

type mockDocumentService struct {
	getWithChunksFn func(ctx context.Context, id string) (*domain.DocumentWithChunks, error)
}

func (m *mockDocumentService) Get(ctx context.Context, id string) (*domain.Document, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDocumentService) GetWithChunks(ctx context.Context, id string) (*domain.DocumentWithChunks, error) {
	if m.getWithChunksFn != nil {
		return m.getWithChunksFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDocumentService) GetContent(ctx context.Context, id string) (*domain.DocumentContent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDocumentService) GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDocumentService) Count(ctx context.Context) (int, error) {
	return 0, errors.New("not implemented")
}

func (m *mockDocumentService) CountBySource(ctx context.Context, sourceID string) (int, error) {
	return 0, errors.New("not implemented")
}

type mockSettingsService struct {
	getFn              func(ctx context.Context) (*domain.Settings, error)
	updateFn           func(ctx context.Context, updaterID string, req driving.UpdateSettingsRequest) (*domain.Settings, error)
	getAISettingsFn    func(ctx context.Context) (*domain.AISettings, error)
	updateAISettingsFn func(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error)
	getAIStatusFn      func(ctx context.Context) (*driving.AISettingsStatus, error)
	testConnectionFn   func(ctx context.Context) error
	getAIProvidersFn   func(ctx context.Context) (*driving.AIProvidersResponse, error)
}

func (m *mockSettingsService) Get(ctx context.Context) (*domain.Settings, error) {
	if m.getFn != nil {
		return m.getFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsService) Update(ctx context.Context, updaterID string, req driving.UpdateSettingsRequest) (*domain.Settings, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, updaterID, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsService) GetAISettings(ctx context.Context) (*domain.AISettings, error) {
	if m.getAISettingsFn != nil {
		return m.getAISettingsFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsService) UpdateAISettings(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
	if m.updateAISettingsFn != nil {
		return m.updateAISettingsFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsService) GetAIStatus(ctx context.Context) (*driving.AISettingsStatus, error) {
	if m.getAIStatusFn != nil {
		return m.getAIStatusFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSettingsService) TestConnection(ctx context.Context) error {
	if m.testConnectionFn != nil {
		return m.testConnectionFn(ctx)
	}
	return errors.New("not implemented")
}

func (m *mockSettingsService) GetAIProviders(ctx context.Context) (*driving.AIProvidersResponse, error) {
	if m.getAIProvidersFn != nil {
		return m.getAIProvidersFn(ctx)
	}
	return nil, errors.New("not implemented")
}

type mockCapabilitiesService struct {
	getCapabilitiesFn func(ctx context.Context) (*driving.CapabilitiesResponse, error)
}

func (m *mockCapabilitiesService) GetCapabilities(ctx context.Context) (*driving.CapabilitiesResponse, error) {
	if m.getCapabilitiesFn != nil {
		return m.getCapabilitiesFn(ctx)
	}
	return &driving.CapabilitiesResponse{
		AIProviders: driving.AIProvidersCapability{
			Embedding: []domain.AIProvider{domain.AIProviderOpenAI},
			LLM:       []domain.AIProvider{domain.AIProviderOpenAI},
		},
	}, nil
}

type mockVespaAdminService struct {
	connectFn     func(ctx context.Context, req driving.ConnectVespaRequest) (*driving.VespaStatus, error)
	statusFn      func(ctx context.Context) (*driving.VespaStatus, error)
	healthCheckFn func(ctx context.Context) error
}

func (m *mockVespaAdminService) Connect(ctx context.Context, req driving.ConnectVespaRequest) (*driving.VespaStatus, error) {
	if m.connectFn != nil {
		return m.connectFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockVespaAdminService) Status(ctx context.Context) (*driving.VespaStatus, error) {
	if m.statusFn != nil {
		return m.statusFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockVespaAdminService) HealthCheck(ctx context.Context) error {
	if m.healthCheckFn != nil {
		return m.healthCheckFn(ctx)
	}
	return errors.New("not implemented")
}

type mockSetupService struct {
	getStatusFn func(ctx context.Context) (*driving.SetupStatusResponse, error)
}

func (m *mockSetupService) GetStatus(ctx context.Context) (*driving.SetupStatusResponse, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx)
	}
	return nil, errors.New("not implemented")
}

func TestHealthHandler(t *testing.T) {
	server := &Server{version: "test"}

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", response.Status)
	}
	if response.Components["server"].Status != "healthy" {
		t.Errorf("expected server component to be healthy")
	}
}

func TestHealthHandler_WithVespaUnhealthy(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		healthCheckFn: func(ctx context.Context) error {
			return errors.New("vespa not connected")
		},
	}
	server := &Server{version: "test", vespaAdminService: mockVespa}

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	// Always returns 200 - service is up and can respond
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %s", response.Status)
	}
	if response.Components["vespa"].Status != "unhealthy" {
		t.Errorf("expected vespa component to be unhealthy")
	}
	if response.Components["server"].Status != "healthy" {
		t.Errorf("expected server component to be healthy")
	}
}

func TestReadyHandler(t *testing.T) {
	server := &Server{version: "test"}

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	server.handleReady(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["status"] != "ready" {
		t.Errorf("expected status 'ready', got %s", response["status"])
	}
}

func TestVersionHandler(t *testing.T) {
	server := &Server{version: "1.2.3"}

	req := httptest.NewRequest("GET", "/version", nil)
	rr := httptest.NewRecorder()

	server.handleVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["version"] != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %s", response["version"])
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]string{"foo": "bar"}
	writeJSON(rr, http.StatusCreated, data)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["foo"] != "bar" {
		t.Errorf("expected foo 'bar', got %s", response["foo"])
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()

	writeError(rr, http.StatusBadRequest, "invalid input")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid input" {
		t.Errorf("expected error 'invalid input', got %s", response["error"])
	}
}

func TestSearchRequest(t *testing.T) {
	reqBody := searchRequest{
		Query:     "test query",
		Mode:      "hybrid",
		Limit:     20,
		Offset:    0,
		SourceIDs: []string{"source-1", "source-2"},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded searchRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Query != "test query" {
		t.Errorf("expected query 'test query', got %s", decoded.Query)
	}
	if decoded.Limit != 20 {
		t.Errorf("expected limit 20, got %d", decoded.Limit)
	}
	if len(decoded.SourceIDs) != 2 {
		t.Errorf("expected 2 source IDs, got %d", len(decoded.SourceIDs))
	}
}

func TestHandleLogin_InvalidJSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleRefresh_InvalidJSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleRefresh(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleLogout_NoToken(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rr := httptest.NewRecorder()

	server.handleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleSearch_InvalidJSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	server := &Server{}

	body, _ := json.Marshal(searchRequest{Query: ""})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "query is required" {
		t.Errorf("expected error 'query is required', got %s", response["error"])
	}
}

func TestHandleCreateUser_InvalidJSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleCreateUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleCreateSource_InvalidJSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/sources", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleCreateSource(rr, req)

	// Should return unauthorized since there's no auth context
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

// Comprehensive Authentication Handler Tests

func TestHandleLogin_Success(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour)
	mockAuth := &mockAuthService{
		authenticateFn: func(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
			if req.Email == "test@example.com" && req.Password == "password123" {
				return &domain.LoginResponse{
					Token:        "test-token",
					RefreshToken: "refresh-token",
					ExpiresAt:    expiresAt,
					User: &domain.UserSummary{
						ID:    "user-1",
						Email: "test@example.com",
						Name:  "Test User",
						Role:  domain.RoleAdmin,
					},
				}, nil
			}
			return nil, domain.ErrInvalidCredentials
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Token != "test-token" {
		t.Errorf("expected token 'test-token', got %s", response.Token)
	}
	if response.User.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", response.User.Email)
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	mockAuth := &mockAuthService{
		authenticateFn: func(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
			return nil, domain.ErrInvalidCredentials
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.LoginRequest{
		Email:    "wrong@example.com",
		Password: "wrongpass",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid credentials" {
		t.Errorf("expected error 'invalid credentials', got %s", response["error"])
	}
}

func TestHandleLogin_AccountDisabled(t *testing.T) {
	mockAuth := &mockAuthService{
		authenticateFn: func(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
			return nil, domain.ErrUnauthorized
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.LoginRequest{
		Email:    "disabled@example.com",
		Password: "password",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "account disabled" {
		t.Errorf("expected error 'account disabled', got %s", response["error"])
	}
}

func TestHandleLogin_InternalError(t *testing.T) {
	mockAuth := &mockAuthService{
		authenticateFn: func(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
			return nil, errors.New("database connection failed")
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.LoginRequest{
		Email:    "test@example.com",
		Password: "password",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleLogin(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleRefresh_Success(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour)
	mockAuth := &mockAuthService{
		refreshTokenFn: func(ctx context.Context, req domain.RefreshRequest) (*domain.LoginResponse, error) {
			return &domain.LoginResponse{
				Token:        "new-token",
				RefreshToken: "new-refresh-token",
				ExpiresAt:    expiresAt,
				User: &domain.UserSummary{
					ID:    "user-1",
					Email: "test@example.com",
				},
			}, nil
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.RefreshRequest{
		RefreshToken: "valid-refresh-token",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleRefresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Token != "new-token" {
		t.Errorf("expected new token, got %s", response.Token)
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	mockAuth := &mockAuthService{
		refreshTokenFn: func(ctx context.Context, req domain.RefreshRequest) (*domain.LoginResponse, error) {
			return nil, domain.ErrTokenExpired
		},
	}

	server := &Server{authService: mockAuth}

	body, _ := json.Marshal(domain.RefreshRequest{
		RefreshToken: "invalid-token",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleRefresh(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleLogout_WithToken(t *testing.T) {
	logoutCalled := false
	mockAuth := &mockAuthService{
		logoutFn: func(ctx context.Context, token string) error {
			logoutCalled = true
			if token == "valid-token" {
				return nil
			}
			return errors.New("invalid token")
		},
	}

	server := &Server{authService: mockAuth}

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	server.handleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !logoutCalled {
		t.Error("logout should have been called")
	}
}

// User Handler Tests

func TestHandleSetup_Success(t *testing.T) {
	mockUser := &mockUserService{
		setupFn: func(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
			return &driving.SetupResponse{
				User: &domain.User{
					ID:    "user-1",
					Email: req.Email,
					Name:  req.Name,
					Role:  domain.RoleAdmin,
				},
				Message: "Setup complete",
			}, nil
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.SetupRequest{
		Email:    "admin@example.com",
		Password: "password123",
		Name:     "Admin User",
	})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSetup(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	var response driving.SetupResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.User.Email != "admin@example.com" {
		t.Errorf("expected email 'admin@example.com', got %s", response.User.Email)
	}
}

func TestHandleSetup_InvalidInput(t *testing.T) {
	mockUser := &mockUserService{
		setupFn: func(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
			return nil, domain.ErrInvalidInput
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.SetupRequest{
		Email:    "",
		Password: "",
		Name:     "",
	})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSetup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleSetup_AlreadyComplete(t *testing.T) {
	mockUser := &mockUserService{
		setupFn: func(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
			return nil, domain.ErrForbidden
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.SetupRequest{
		Email:    "admin@example.com",
		Password: "password",
		Name:     "Admin",
	})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSetup(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestHandleSetup_InternalError(t *testing.T) {
	mockUser := &mockUserService{
		setupFn: func(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.SetupRequest{
		Email:    "admin@example.com",
		Password: "password",
		Name:     "Admin",
	})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSetup(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleGetMe_Success(t *testing.T) {
	mockUser := &mockUserService{
		getFn: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{
				ID:     id,
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   domain.RoleAdmin,
				Active: true,
			}, nil
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	authCtx := &domain.AuthContext{
		UserID: "user-1",
		Email:  "test@example.com",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleGetMe(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.UserSummary
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", response.Email)
	}
}

func TestHandleGetMe_NoAuthContext(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	rr := httptest.NewRecorder()

	server.handleGetMe(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleGetMe_UserNotFound(t *testing.T) {
	mockUser := &mockUserService{
		getFn: func(ctx context.Context, id string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	authCtx := &domain.AuthContext{
		UserID: "nonexistent",
		Email:  "test@example.com",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleGetMe(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleListUsers_Success(t *testing.T) {
	mockUser := &mockUserService{
		listFn: func(ctx context.Context) ([]*domain.User, error) {
			return []*domain.User{
				{
					ID:     "user-1",
					Email:  "user1@example.com",
					Name:   "User 1",
					Role:   domain.RoleAdmin,
					Active: true,
				},
				{
					ID:     "user-2",
					Email:  "user2@example.com",
					Name:   "User 2",
					Role:   domain.RoleMember,
					Active: true,
				},
			}, nil
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()

	server.handleListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response []*domain.UserSummary
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response) != 2 {
		t.Errorf("expected 2 users, got %d", len(response))
	}
}

func TestHandleListUsers_Error(t *testing.T) {
	mockUser := &mockUserService{
		listFn: func(ctx context.Context) ([]*domain.User, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()

	server.handleListUsers(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleCreateUser_Success(t *testing.T) {
	mockUser := &mockUserService{
		createFn: func(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error) {
			return &domain.User{
				ID:     "user-new",
				Email:  req.Email,
				Name:   req.Name,
				Role:   req.Role,
				Active: true,
			}, nil
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.CreateUserRequest{
		Email:    "newuser@example.com",
		Password: "password123",
		Name:     "New User",
		Role:     domain.RoleMember,
	})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleCreateUser(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	var response domain.UserSummary
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Email != "newuser@example.com" {
		t.Errorf("expected email 'newuser@example.com', got %s", response.Email)
	}
}

func TestHandleCreateUser_AlreadyExists(t *testing.T) {
	mockUser := &mockUserService{
		createFn: func(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error) {
			return nil, domain.ErrAlreadyExists
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.CreateUserRequest{
		Email:    "existing@example.com",
		Password: "password",
		Name:     "User",
		Role:     domain.RoleMember,
	})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleCreateUser(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestHandleCreateUser_InternalError(t *testing.T) {
	mockUser := &mockUserService{
		createFn: func(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{userService: mockUser}

	body, _ := json.Marshal(driving.CreateUserRequest{
		Email:    "user@example.com",
		Password: "password",
		Name:     "User",
		Role:     domain.RoleMember,
	})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleCreateUser(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleDeleteUser_Success(t *testing.T) {
	mockUser := &mockUserService{
		deleteFn: func(ctx context.Context, id string) error {
			if id == "user-1" {
				return nil
			}
			return domain.ErrNotFound
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("DELETE", "/api/v1/users/user-1", nil)
	req.SetPathValue("id", "user-1")
	rr := httptest.NewRecorder()

	server.handleDeleteUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleDeleteUser_NotFound(t *testing.T) {
	mockUser := &mockUserService{
		deleteFn: func(ctx context.Context, id string) error {
			return domain.ErrNotFound
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("DELETE", "/api/v1/users/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()

	server.handleDeleteUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleDeleteUser_MissingID(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("DELETE", "/api/v1/users/", nil)
	rr := httptest.NewRecorder()

	server.handleDeleteUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleDeleteUser_InternalError(t *testing.T) {
	mockUser := &mockUserService{
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("database error")
		},
	}

	server := &Server{userService: mockUser}

	req := httptest.NewRequest("DELETE", "/api/v1/users/user-1", nil)
	req.SetPathValue("id", "user-1")
	rr := httptest.NewRecorder()

	server.handleDeleteUser(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// Search Handler Tests

func TestHandleSearch_Success(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return &domain.SearchResult{
				Query:      query,
				TotalCount: 10,
				Took:       50 * time.Millisecond,
				Mode:       opts.Mode,
				Results:    []*domain.RankedChunk{},
			}, nil
		},
	}

	server := &Server{searchService: mockSearch}

	body, _ := json.Marshal(searchRequest{
		Query:  "test query",
		Mode:   domain.SearchModeHybrid,
		Limit:  20,
		Offset: 0,
	})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.SearchResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Query != "test query" {
		t.Errorf("expected query 'test query', got %s", response.Query)
	}
}

func TestHandleSearch_ServiceError(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return nil, errors.New("search service unavailable")
		},
	}

	server := &Server{searchService: mockSearch}

	body, _ := json.Marshal(searchRequest{
		Query: "test query",
	})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// Source Handler Tests

func TestHandleListSources_Success(t *testing.T) {
	mockSource := &mockSourceService{
		listWithSummaryFn: func(ctx context.Context) ([]*domain.SourceSummary, error) {
			return []*domain.SourceSummary{
				{
					Source: &domain.Source{
						ID:           "source-1",
						Name:         "Source 1",
						ProviderType: domain.ProviderTypeGitHub,
					},
					DocumentCount: 100,
					SyncStatus:    "idle",
				},
			}, nil
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("GET", "/api/v1/sources", nil)
	rr := httptest.NewRecorder()

	server.handleListSources(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response []*domain.SourceSummary
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response) != 1 {
		t.Errorf("expected 1 source, got %d", len(response))
	}
}

func TestHandleListSources_Error(t *testing.T) {
	mockSource := &mockSourceService{
		listWithSummaryFn: func(ctx context.Context) ([]*domain.SourceSummary, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("GET", "/api/v1/sources", nil)
	rr := httptest.NewRecorder()

	server.handleListSources(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleGetSource_Success(t *testing.T) {
	mockSource := &mockSourceService{
		getFn: func(ctx context.Context, id string) (*domain.Source, error) {
			if id == "source-1" {
				return &domain.Source{
					ID:           id,
					Name:         "Test Source",
					ProviderType: domain.ProviderTypeGitHub,
					Enabled:      true,
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("GET", "/api/v1/sources/source-1", nil)
	req.SetPathValue("id", "source-1")
	rr := httptest.NewRecorder()

	server.handleGetSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.Source
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Name != "Test Source" {
		t.Errorf("expected name 'Test Source', got %s", response.Name)
	}
}

func TestHandleGetSource_NotFound(t *testing.T) {
	mockSource := &mockSourceService{
		getFn: func(ctx context.Context, id string) (*domain.Source, error) {
			return nil, domain.ErrNotFound
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("GET", "/api/v1/sources/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()

	server.handleGetSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleGetSource_MissingID(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/sources/", nil)
	rr := httptest.NewRecorder()

	server.handleGetSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGetSource_InternalError(t *testing.T) {
	mockSource := &mockSourceService{
		getFn: func(ctx context.Context, id string) (*domain.Source, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("GET", "/api/v1/sources/source-1", nil)
	req.SetPathValue("id", "source-1")
	rr := httptest.NewRecorder()

	server.handleGetSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleCreateSource_Success(t *testing.T) {
	mockSource := &mockSourceService{
		createFn: func(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error) {
			return &domain.Source{
				ID:           "source-new",
				Name:         req.Name,
				ProviderType: req.ProviderType,
				CreatedBy:    creatorID,
				Enabled:      true,
			}, nil
		},
	}

	server := &Server{sourceService: mockSource}

	body, _ := json.Marshal(driving.CreateSourceRequest{
		Name:         "New Source",
		ProviderType: domain.ProviderTypeGitHub,
		Config:       domain.SourceConfig{},
	})
	req := httptest.NewRequest("POST", "/api/v1/sources", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-1",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleCreateSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	var response domain.Source
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Name != "New Source" {
		t.Errorf("expected name 'New Source', got %s", response.Name)
	}
}

func TestHandleCreateSource_AlreadyExists(t *testing.T) {
	mockSource := &mockSourceService{
		createFn: func(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error) {
			return nil, domain.ErrAlreadyExists
		},
	}

	server := &Server{sourceService: mockSource}

	body, _ := json.Marshal(driving.CreateSourceRequest{
		Name:         "Existing Source",
		ProviderType: domain.ProviderTypeGitHub,
	})
	req := httptest.NewRequest("POST", "/api/v1/sources", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{UserID: "user-1", Role: domain.RoleAdmin}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleCreateSource(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestHandleCreateSource_InternalError(t *testing.T) {
	mockSource := &mockSourceService{
		createFn: func(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{sourceService: mockSource}

	body, _ := json.Marshal(driving.CreateSourceRequest{
		Name:         "New Source",
		ProviderType: domain.ProviderTypeGitHub,
	})
	req := httptest.NewRequest("POST", "/api/v1/sources", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{UserID: "user-1", Role: domain.RoleAdmin}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleCreateSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleDeleteSource_Success(t *testing.T) {
	mockSource := &mockSourceService{
		deleteFn: func(ctx context.Context, id string) error {
			if id == "source-1" {
				return nil
			}
			return domain.ErrNotFound
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("DELETE", "/api/v1/sources/source-1", nil)
	req.SetPathValue("id", "source-1")
	rr := httptest.NewRecorder()

	server.handleDeleteSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleDeleteSource_NotFound(t *testing.T) {
	mockSource := &mockSourceService{
		deleteFn: func(ctx context.Context, id string) error {
			return domain.ErrNotFound
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("DELETE", "/api/v1/sources/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()

	server.handleDeleteSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleDeleteSource_MissingID(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("DELETE", "/api/v1/sources/", nil)
	rr := httptest.NewRecorder()

	server.handleDeleteSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleDeleteSource_InternalError(t *testing.T) {
	mockSource := &mockSourceService{
		deleteFn: func(ctx context.Context, id string) error {
			return errors.New("database error")
		},
	}

	server := &Server{sourceService: mockSource}

	req := httptest.NewRequest("DELETE", "/api/v1/sources/source-1", nil)
	req.SetPathValue("id", "source-1")
	rr := httptest.NewRecorder()

	server.handleDeleteSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// mockTaskQueue implements driven.TaskQueue for testing
type mockTaskQueue struct {
	enqueueFn func(ctx context.Context, task *domain.Task) error
}

func (m *mockTaskQueue) Enqueue(ctx context.Context, task *domain.Task) error {
	if m.enqueueFn != nil {
		return m.enqueueFn(ctx, task)
	}
	return nil
}

func (m *mockTaskQueue) EnqueueBatch(ctx context.Context, tasks []*domain.Task) error {
	return nil
}

func (m *mockTaskQueue) Dequeue(ctx context.Context) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTaskQueue) DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTaskQueue) Ack(ctx context.Context, taskID string) error {
	return nil
}

func (m *mockTaskQueue) Nack(ctx context.Context, taskID string, reason string) error {
	return nil
}

func (m *mockTaskQueue) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTaskQueue) ListTasks(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTaskQueue) CancelTask(ctx context.Context, taskID string) error {
	return nil
}

func (m *mockTaskQueue) PurgeTasks(ctx context.Context, olderThan int) (int, error) {
	return 0, nil
}

func (m *mockTaskQueue) Stats(ctx context.Context) (*driven.QueueStats, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTaskQueue) Ping(ctx context.Context) error {
	return nil
}

func (m *mockTaskQueue) Close() error {
	return nil
}

func TestHandleTriggerSync_Success(t *testing.T) {
	mockSource := &mockSourceService{
		getFn: func(ctx context.Context, id string) (*domain.Source, error) {
			return &domain.Source{
				ID:           id,
				Name:         "Test Source",
				ProviderType: domain.ProviderTypeGitHub,
			}, nil
		},
	}
	mockQueue := &mockTaskQueue{
		enqueueFn: func(ctx context.Context, task *domain.Task) error {
			return nil
		},
	}

	server := &Server{
		sourceService: mockSource,
		taskQueue:     mockQueue,
	}

	req := httptest.NewRequest("POST", "/api/v1/sources/source-1/sync", nil)
	req.SetPathValue("id", "source-1")
	rr := httptest.NewRecorder()

	server.handleTriggerSync(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["status"] != "accepted" {
		t.Errorf("expected status 'accepted', got %s", response["status"])
	}
}

func TestHandleTriggerSync_MissingID(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/sources//sync", nil)
	// No path value set for "id"
	rr := httptest.NewRecorder()

	server.handleTriggerSync(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// Document Handler Tests

func TestHandleGetDocument_Success(t *testing.T) {
	mockDoc := &mockDocumentService{
		getWithChunksFn: func(ctx context.Context, id string) (*domain.DocumentWithChunks, error) {
			if id == "doc-1" {
				return &domain.DocumentWithChunks{
					Document: &domain.Document{
						ID:    id,
						Title: "Test Document",
					},
					Chunks: []*domain.Chunk{},
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	server := &Server{docService: mockDoc}

	req := httptest.NewRequest("GET", "/api/v1/documents/doc-1", nil)
	req.SetPathValue("id", "doc-1")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.DocumentWithChunks
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Document.Title != "Test Document" {
		t.Errorf("expected title 'Test Document', got %s", response.Document.Title)
	}
}

func TestHandleGetDocument_NotFound(t *testing.T) {
	mockDoc := &mockDocumentService{
		getWithChunksFn: func(ctx context.Context, id string) (*domain.DocumentWithChunks, error) {
			return nil, domain.ErrNotFound
		},
	}

	server := &Server{docService: mockDoc}

	req := httptest.NewRequest("GET", "/api/v1/documents/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleGetDocument_MissingID(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/documents/", nil)
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleGetDocument_InternalError(t *testing.T) {
	mockDoc := &mockDocumentService{
		getWithChunksFn: func(ctx context.Context, id string) (*domain.DocumentWithChunks, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{docService: mockDoc}

	req := httptest.NewRequest("GET", "/api/v1/documents/doc-1", nil)
	req.SetPathValue("id", "doc-1")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// Settings Handler Tests

func TestHandleGetSettings_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		getFn: func(ctx context.Context) (*domain.Settings, error) {
			return &domain.Settings{
				TeamID:            "team-1",
				DefaultSearchMode: domain.SearchModeHybrid,
				ResultsPerPage:    20,
			}, nil
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings", nil)
	rr := httptest.NewRecorder()

	server.handleGetSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response domain.Settings
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.DefaultSearchMode != domain.SearchModeHybrid {
		t.Errorf("expected search mode 'hybrid', got %s", response.DefaultSearchMode)
	}
}

func TestHandleGetSettings_Error(t *testing.T) {
	mockSettings := &mockSettingsService{
		getFn: func(ctx context.Context) (*domain.Settings, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings", nil)
	rr := httptest.NewRecorder()

	server.handleGetSettings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleUpdateSettings_Success(t *testing.T) {
	resultsPerPage := 50
	mockSettings := &mockSettingsService{
		updateFn: func(ctx context.Context, updaterID string, req driving.UpdateSettingsRequest) (*domain.Settings, error) {
			return &domain.Settings{
				TeamID:         "team-1",
				ResultsPerPage: *req.ResultsPerPage,
			}, nil
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateSettingsRequest{
		ResultsPerPage: &resultsPerPage,
	})
	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{UserID: "user-1", Role: domain.RoleAdmin}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleUpdateSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleUpdateSettings_NoAuthContext(t *testing.T) {
	server := &Server{}

	body, _ := json.Marshal(driving.UpdateSettingsRequest{})
	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleUpdateSettings(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleUpdateSettings_InvalidJSON(t *testing.T) {
	server := &Server{settingsService: &mockSettingsService{}}

	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewBufferString("invalid json"))
	authCtx := &domain.AuthContext{UserID: "user-1", Role: domain.RoleAdmin}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleUpdateSettings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleUpdateSettings_InternalError(t *testing.T) {
	resultsPerPage := 50
	mockSettings := &mockSettingsService{
		updateFn: func(ctx context.Context, updaterID string, req driving.UpdateSettingsRequest) (*domain.Settings, error) {
			return nil, errors.New("database error")
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateSettingsRequest{
		ResultsPerPage: &resultsPerPage,
	})
	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{UserID: "user-1", Role: domain.RoleAdmin}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleUpdateSettings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleGetAISettings_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAISettingsFn: func(ctx context.Context) (*domain.AISettings, error) {
			return &domain.AISettings{
				TeamID: "team-1",
				Embedding: domain.EmbeddingSettings{
					Provider: domain.AIProviderOpenAI,
					Model:    "text-embedding-3-small",
				},
				LLM: domain.LLMSettings{
					Provider: domain.AIProviderOpenAI,
					Model:    "gpt-4",
				},
			}, nil
		},
	}

	mockCapabilities := &mockCapabilitiesService{}

	server := &Server{
		settingsService:      mockSettings,
		capabilitiesService: mockCapabilities,
	}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai", nil)
	rr := httptest.NewRecorder()

	server.handleGetAISettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response aiSettingsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Embedding.Provider != domain.AIProviderOpenAI {
		t.Errorf("expected provider 'openai', got %s", response.Embedding.Provider)
	}
	if !response.Embedding.HasAPIKey {
		t.Error("expected HasAPIKey to be true")
	}
}

func TestHandleGetAISettings_Error(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAISettingsFn: func(ctx context.Context) (*domain.AISettings, error) {
			return nil, errors.New("settings not found")
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai", nil)
	rr := httptest.NewRecorder()

	server.handleGetAISettings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleUpdateAISettings_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		updateAISettingsFn: func(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
			return &driving.AISettingsStatus{
				Embedding: driving.AIServiceStatus{
					Available: true,
					Provider:  domain.AIProviderOpenAI,
					Model:     "text-embedding-3-small",
				},
			}, nil
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateAISettingsRequest{
		Embedding: &driving.EmbeddingSettingsInput{
			Provider: domain.AIProviderOpenAI,
			Model:    "text-embedding-3-small",
		},
	})
	req := httptest.NewRequest("PUT", "/api/v1/settings/ai", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleUpdateAISettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleUpdateAISettings_InvalidProvider(t *testing.T) {
	mockSettings := &mockSettingsService{
		updateAISettingsFn: func(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
			return nil, domain.ErrInvalidProvider
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateAISettingsRequest{})
	req := httptest.NewRequest("PUT", "/api/v1/settings/ai", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleUpdateAISettings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleUpdateAISettings_InvalidInput(t *testing.T) {
	mockSettings := &mockSettingsService{
		updateAISettingsFn: func(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
			return nil, domain.ErrInvalidInput
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateAISettingsRequest{})
	req := httptest.NewRequest("PUT", "/api/v1/settings/ai", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleUpdateAISettings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleUpdateAISettings_InternalError(t *testing.T) {
	mockSettings := &mockSettingsService{
		updateAISettingsFn: func(ctx context.Context, req driving.UpdateAISettingsRequest) (*driving.AISettingsStatus, error) {
			return nil, errors.New("internal error")
		},
	}

	server := &Server{settingsService: mockSettings}

	body, _ := json.Marshal(driving.UpdateAISettingsRequest{})
	req := httptest.NewRequest("PUT", "/api/v1/settings/ai", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleUpdateAISettings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleGetAIStatus_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAIStatusFn: func(ctx context.Context) (*driving.AISettingsStatus, error) {
			return &driving.AISettingsStatus{
				Embedding: driving.AIServiceStatus{
					Available: true,
					Provider:  domain.AIProviderOpenAI,
				},
				LLM: driving.AIServiceStatus{
					Available: true,
					Provider:  domain.AIProviderOpenAI,
				},
			}, nil
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai/status", nil)
	rr := httptest.NewRecorder()

	server.handleGetAIStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleGetAIStatus_WithVespaService(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAIStatusFn: func(ctx context.Context) (*driving.AISettingsStatus, error) {
			return &driving.AISettingsStatus{
				Embedding: driving.AIServiceStatus{Available: true},
				LLM:       driving.AIServiceStatus{Available: true},
			}, nil
		},
	}

	mockVespa := &mockVespaAdminService{
		statusFn: func(ctx context.Context) (*driving.VespaStatus, error) {
			return &driving.VespaStatus{
				Connected:         true,
				SchemaMode:        domain.VespacSchemaModeHybrid,
				EmbeddingsEnabled: true,
				EmbeddingDim:      384,
				Healthy:           true,
			}, nil
		},
	}

	server := &Server{
		settingsService:   mockSettings,
		vespaAdminService: mockVespa,
	}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai/status", nil)
	rr := httptest.NewRecorder()

	server.handleGetAIStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response driving.AISettingsStatus
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Vespa.Connected {
		t.Error("expected Vespa to be connected")
	}
	if response.Vespa.EmbeddingDim != 384 {
		t.Errorf("expected embedding dim 384, got %d", response.Vespa.EmbeddingDim)
	}
}

func TestHandleTestAIConnection_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		testConnectionFn: func(ctx context.Context) error {
			return nil
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("POST", "/api/v1/settings/ai/test", nil)
	rr := httptest.NewRecorder()

	server.handleTestAIConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleTestAIConnection_Failure(t *testing.T) {
	mockSettings := &mockSettingsService{
		testConnectionFn: func(ctx context.Context) error {
			return errors.New("connection failed")
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("POST", "/api/v1/settings/ai/test", nil)
	rr := httptest.NewRecorder()

	server.handleTestAIConnection(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

// Vespa Admin Handler Tests

func TestHandleVespaConnect_Success(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		connectFn: func(ctx context.Context, req driving.ConnectVespaRequest) (*driving.VespaStatus, error) {
			return &driving.VespaStatus{
				Connected:  true,
				Endpoint:   "http://localhost:8080",
				DevMode:    req.DevMode,
				SchemaMode: domain.VespacSchemaModeBM25,
				Healthy:    true,
			}, nil
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	body, _ := json.Marshal(driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/vespa/connect", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleVespaConnect(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response driving.VespaStatus
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Connected {
		t.Error("expected Vespa to be connected")
	}
}

func TestHandleVespaConnect_EmptyBody(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		connectFn: func(ctx context.Context, req driving.ConnectVespaRequest) (*driving.VespaStatus, error) {
			return &driving.VespaStatus{Connected: true}, nil
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	req := httptest.NewRequest("POST", "/api/v1/admin/vespa/connect", nil)
	rr := httptest.NewRecorder()

	server.handleVespaConnect(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleVespaConnect_InvalidJSON(t *testing.T) {
	server := &Server{vespaAdminService: &mockVespaAdminService{}}

	req := httptest.NewRequest("POST", "/api/v1/admin/vespa/connect", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()

	server.handleVespaConnect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleVespaConnect_Error(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		connectFn: func(ctx context.Context, req driving.ConnectVespaRequest) (*driving.VespaStatus, error) {
			return nil, errors.New("connection failed")
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	body, _ := json.Marshal(driving.ConnectVespaRequest{
		Endpoint: "http://localhost:8080",
		DevMode:  true,
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/vespa/connect", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleVespaConnect(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleVespaStatus_Success(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		statusFn: func(ctx context.Context) (*driving.VespaStatus, error) {
			return &driving.VespaStatus{
				Connected:  true,
				SchemaMode: domain.VespacSchemaModeHybrid,
				Healthy:    true,
			}, nil
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	req := httptest.NewRequest("GET", "/api/v1/admin/vespa/status", nil)
	rr := httptest.NewRecorder()

	server.handleVespaStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleVespaStatus_Error(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		statusFn: func(ctx context.Context) (*driving.VespaStatus, error) {
			return nil, errors.New("vespa not configured")
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	req := httptest.NewRequest("GET", "/api/v1/admin/vespa/status", nil)
	rr := httptest.NewRecorder()

	server.handleVespaStatus(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleVespaHealth_Success(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		healthCheckFn: func(ctx context.Context) error {
			return nil
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	req := httptest.NewRequest("GET", "/api/v1/admin/vespa/health", nil)
	rr := httptest.NewRecorder()

	server.handleVespaHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleVespaHealth_Unhealthy(t *testing.T) {
	mockVespa := &mockVespaAdminService{
		healthCheckFn: func(ctx context.Context) error {
			return errors.New("vespa cluster unhealthy")
		},
	}

	server := &Server{vespaAdminService: mockVespa}

	req := httptest.NewRequest("GET", "/api/v1/admin/vespa/health", nil)
	rr := httptest.NewRecorder()

	server.handleVespaHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

// Setup Handler Tests

func TestHandleSetupStatus_Success(t *testing.T) {
	mockSetup := &mockSetupService{
		getStatusFn: func(ctx context.Context) (*driving.SetupStatusResponse, error) {
			return &driving.SetupStatusResponse{
				SetupComplete:  true,
				HasUsers:       true,
				HasSources:     true,
				VespaConnected: true,
			}, nil
		},
	}

	server := &Server{setupService: mockSetup}

	req := httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	rr := httptest.NewRecorder()

	server.handleSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response driving.SetupStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.SetupComplete {
		t.Error("expected setup to be complete")
	}
	if !response.HasUsers {
		t.Error("expected HasUsers to be true")
	}
	if !response.HasSources {
		t.Error("expected HasSources to be true")
	}
	if !response.VespaConnected {
		t.Error("expected VespaConnected to be true")
	}
}

func TestHandleSetupStatus_SetupIncomplete(t *testing.T) {
	mockSetup := &mockSetupService{
		getStatusFn: func(ctx context.Context) (*driving.SetupStatusResponse, error) {
			return &driving.SetupStatusResponse{
				SetupComplete:  false,
				HasUsers:       false,
				HasSources:     false,
				VespaConnected: false,
			}, nil
		},
	}

	server := &Server{setupService: mockSetup}

	req := httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	rr := httptest.NewRecorder()

	server.handleSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response driving.SetupStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.SetupComplete {
		t.Error("expected setup to be incomplete")
	}
	if response.HasUsers {
		t.Error("expected HasUsers to be false")
	}
	if response.HasSources {
		t.Error("expected HasSources to be false")
	}
	if response.VespaConnected {
		t.Error("expected VespaConnected to be false")
	}
}

func TestHandleSetupStatus_ServiceUnavailable(t *testing.T) {
	server := &Server{setupService: nil}

	req := httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	rr := httptest.NewRecorder()

	server.handleSetupStatus(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleSetupStatus_Error(t *testing.T) {
	mockSetup := &mockSetupService{
		getStatusFn: func(ctx context.Context) (*driving.SetupStatusResponse, error) {
			return nil, errors.New("database connection failed")
		},
	}

	server := &Server{setupService: mockSetup}

	req := httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	rr := httptest.NewRecorder()

	server.handleSetupStatus(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// AI Providers Handler Tests

func TestHandleGetAIProviders_Success(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAIProvidersFn: func(ctx context.Context) (*driving.AIProvidersResponse, error) {
			return &driving.AIProvidersResponse{
				Embedding: []domain.AIProviderInfo{
					{
						ID:   string(domain.AIProviderOpenAI),
						Name: "OpenAI",
						Models: []domain.AIModelInfo{
							{
								ID:         "text-embedding-3-small",
								Name:       "Text Embedding 3 Small",
								Dimensions: 1536,
							},
						},
						RequiresAPIKey:  true,
						RequiresBaseURL: false,
						APIKeyURL:       "https://platform.openai.com/api-keys",
					},
				},
				LLM: []domain.AIProviderInfo{
					{
						ID:   string(domain.AIProviderOpenAI),
						Name: "OpenAI",
						Models: []domain.AIModelInfo{
							{
								ID:   "gpt-4o",
								Name: "GPT-4o",
							},
						},
						RequiresAPIKey:  true,
						RequiresBaseURL: false,
						APIKeyURL:       "https://platform.openai.com/api-keys",
					},
				},
			}, nil
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai/providers", nil)
	rr := httptest.NewRecorder()

	server.handleGetAIProviders(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response driving.AIProvidersResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Embedding) == 0 {
		t.Error("expected at least one embedding provider")
	}
	if len(response.LLM) == 0 {
		t.Error("expected at least one LLM provider")
	}

	// Verify OpenAI embedding provider
	if response.Embedding[0].ID != string(domain.AIProviderOpenAI) {
		t.Errorf("expected provider ID 'openai', got %s", response.Embedding[0].ID)
	}
	if response.Embedding[0].Name != "OpenAI" {
		t.Errorf("expected provider name 'OpenAI', got %s", response.Embedding[0].Name)
	}
	if !response.Embedding[0].RequiresAPIKey {
		t.Error("expected RequiresAPIKey to be true")
	}
	if response.Embedding[0].RequiresBaseURL {
		t.Error("expected RequiresBaseURL to be false")
	}
	if response.Embedding[0].APIKeyURL == "" {
		t.Error("expected APIKeyURL to be set")
	}
	if len(response.Embedding[0].Models) == 0 {
		t.Error("expected at least one model")
	}
	if response.Embedding[0].Models[0].Dimensions == 0 {
		t.Error("expected model to have dimensions")
	}

	// Verify OpenAI LLM provider
	if response.LLM[0].ID != string(domain.AIProviderOpenAI) {
		t.Errorf("expected provider ID 'openai', got %s", response.LLM[0].ID)
	}
	if len(response.LLM[0].Models) == 0 {
		t.Error("expected at least one model")
	}
}

func TestHandleGetAIProviders_Error(t *testing.T) {
	mockSettings := &mockSettingsService{
		getAIProvidersFn: func(ctx context.Context) (*driving.AIProvidersResponse, error) {
			return nil, errors.New("internal error")
		},
	}

	server := &Server{settingsService: mockSettings}

	req := httptest.NewRequest("GET", "/api/v1/settings/ai/providers", nil)
	rr := httptest.NewRecorder()

	server.handleGetAIProviders(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}
