package services

import (
	"context"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// mockOAuthStateStore implements driven.OAuthStateStore for testing
type mockOAuthStateStore struct {
	states map[string]*driven.OAuthState
}

func newMockOAuthStateStore() *mockOAuthStateStore {
	return &mockOAuthStateStore{
		states: make(map[string]*driven.OAuthState),
	}
}

func (m *mockOAuthStateStore) Save(ctx context.Context, state *driven.OAuthState) error {
	m.states[state.State] = state
	return nil
}

func (m *mockOAuthStateStore) GetAndDelete(ctx context.Context, state string) (*driven.OAuthState, error) {
	s, ok := m.states[state]
	if !ok {
		return nil, nil
	}
	delete(m.states, state)
	// Check expiry
	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	return s, nil
}

func (m *mockOAuthStateStore) Cleanup(ctx context.Context) error {
	now := time.Now()
	for k, v := range m.states {
		if now.After(v.ExpiresAt) {
			delete(m.states, k)
		}
	}
	return nil
}

// mockInstallationStore implements driven.InstallationStore for testing
type mockInstallationStore struct {
	installations map[string]*domain.Installation
	byAccount     map[string]*domain.Installation // providerType:accountID -> Installation
}

func newMockInstallationStore() *mockInstallationStore {
	return &mockInstallationStore{
		installations: make(map[string]*domain.Installation),
		byAccount:     make(map[string]*domain.Installation),
	}
}

func (m *mockInstallationStore) Save(ctx context.Context, inst *domain.Installation) error {
	m.installations[inst.ID] = inst
	key := string(inst.ProviderType) + ":" + inst.AccountID
	m.byAccount[key] = inst
	return nil
}

func (m *mockInstallationStore) Get(ctx context.Context, id string) (*domain.Installation, error) {
	inst, ok := m.installations[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return inst, nil
}

func (m *mockInstallationStore) List(ctx context.Context) ([]*domain.InstallationSummary, error) {
	summaries := make([]*domain.InstallationSummary, 0, len(m.installations))
	for _, inst := range m.installations {
		summaries = append(summaries, inst.ToSummary())
	}
	return summaries, nil
}

func (m *mockInstallationStore) Delete(ctx context.Context, id string) error {
	inst, ok := m.installations[id]
	if !ok {
		return domain.ErrNotFound
	}
	key := string(inst.ProviderType) + ":" + inst.AccountID
	delete(m.byAccount, key)
	delete(m.installations, id)
	return nil
}

func (m *mockInstallationStore) GetByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.InstallationSummary, error) {
	var summaries []*domain.InstallationSummary
	for _, inst := range m.installations {
		if inst.ProviderType == providerType {
			summaries = append(summaries, inst.ToSummary())
		}
	}
	return summaries, nil
}

func (m *mockInstallationStore) GetByAccountID(ctx context.Context, providerType domain.ProviderType, accountID string) (*domain.Installation, error) {
	key := string(providerType) + ":" + accountID
	inst, ok := m.byAccount[key]
	if !ok {
		return nil, nil
	}
	return inst, nil
}

func (m *mockInstallationStore) UpdateSecrets(ctx context.Context, id string, secrets *domain.InstallationSecrets, expiry *time.Time) error {
	inst, ok := m.installations[id]
	if !ok {
		return domain.ErrNotFound
	}
	inst.Secrets = secrets
	inst.OAuthExpiry = expiry
	inst.UpdatedAt = time.Now()
	return nil
}

func (m *mockInstallationStore) UpdateLastUsed(ctx context.Context, id string) error {
	inst, ok := m.installations[id]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	inst.LastUsedAt = &now
	return nil
}

// mockConfigProvider implements driven.ConfigProvider for testing
type mockConfigProvider struct {
	oauthCredentials map[domain.ProviderType]*driven.OAuthCredentials
	aiCredentials    map[domain.AIProvider]*driven.AICredentials
	baseURL          string
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		oauthCredentials: make(map[domain.ProviderType]*driven.OAuthCredentials),
		aiCredentials:    make(map[domain.AIProvider]*driven.AICredentials),
		baseURL:          "http://localhost:3000",
	}
}

func (m *mockConfigProvider) GetOAuthCredentials(provider domain.ProviderType) *driven.OAuthCredentials {
	return m.oauthCredentials[provider]
}

func (m *mockConfigProvider) GetAICredentials(provider domain.AIProvider) *driven.AICredentials {
	return m.aiCredentials[provider]
}

func (m *mockConfigProvider) IsOAuthConfigured(provider domain.ProviderType) bool {
	return m.oauthCredentials[provider] != nil
}

func (m *mockConfigProvider) IsAIConfigured(provider domain.AIProvider) bool {
	return m.aiCredentials[provider] != nil
}

func (m *mockConfigProvider) GetCapabilities() *driven.Capabilities {
	oauthProviders := []domain.ProviderType{}
	for k := range m.oauthCredentials {
		oauthProviders = append(oauthProviders, k)
	}
	embeddingProviders := []domain.AIProvider{}
	llmProviders := []domain.AIProvider{}
	for k := range m.aiCredentials {
		embeddingProviders = append(embeddingProviders, k)
		llmProviders = append(llmProviders, k)
	}
	return &driven.Capabilities{
		OAuthProviders:     oauthProviders,
		EmbeddingProviders: embeddingProviders,
		LLMProviders:       llmProviders,
	}
}

func (m *mockConfigProvider) GetBaseURL() string {
	return m.baseURL
}

func (m *mockConfigProvider) GetJWTSecret() string {
	return "test-jwt-secret"
}

func (m *mockConfigProvider) GetMasterKey() []byte {
	return []byte("test-master-key-32-bytes-long!!")
}

func (m *mockConfigProvider) GetDatabaseURL() string {
	return "postgres://test"
}

func (m *mockConfigProvider) GetVespaConfigURL() string {
	return "http://localhost:19071"
}

func (m *mockConfigProvider) GetVespaContainerURL() string {
	return "http://localhost:8080"
}

// mockOAuthHandler implements connectors.OAuthHandler for testing
type mockOAuthHandler struct {
	authURL   string
	tokenURL  string
	scopes    []string
	userID    string
	userName  string
	userEmail string
}

func newMockOAuthHandler() *mockOAuthHandler {
	return &mockOAuthHandler{
		authURL:   "https://github.com/login/oauth/authorize",
		tokenURL:  "https://github.com/login/oauth/access_token",
		scopes:    []string{"repo", "read:user"},
		userID:    "12345",
		userName:  "testuser",
		userEmail: "test@example.com",
	}
}

func (m *mockOAuthHandler) BuildAuthURL(clientID, redirectURI, state, codeChallenge string, scopes []string) string {
	return m.authURL + "?client_id=" + clientID + "&state=" + state
}

func (m *mockOAuthHandler) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
	return &driven.OAuthToken{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "repo read:user",
	}, nil
}

func (m *mockOAuthHandler) RefreshToken(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
	return &driven.OAuthToken{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "repo read:user",
	}, nil
}

func (m *mockOAuthHandler) GetUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	return &driven.OAuthUserInfo{
		ID:    m.userID,
		Name:  m.userName,
		Email: m.userEmail,
	}, nil
}

func (m *mockOAuthHandler) DefaultConfig() driven.OAuthConfig {
	return driven.OAuthConfig{
		AuthURL:  m.authURL,
		TokenURL: m.tokenURL,
		Scopes:   m.scopes,
	}
}

// mockOAuthHandlerFactory implements driven.OAuthHandlerFactory for testing
type mockOAuthHandlerFactory struct {
	handlers map[domain.ProviderType]driven.OAuthHandler
}

func newMockOAuthHandlerFactory() *mockOAuthHandlerFactory {
	return &mockOAuthHandlerFactory{
		handlers: make(map[domain.ProviderType]driven.OAuthHandler),
	}
}

func (m *mockOAuthHandlerFactory) GetOAuthHandler(providerType domain.ProviderType) driven.OAuthHandler {
	return m.handlers[providerType]
}

func TestOAuthService_Authorize(t *testing.T) {
	configProvider := newMockConfigProvider()
	oauthStateStore := newMockOAuthStateStore()
	installStore := newMockInstallationStore()
	handlerFactory := newMockOAuthHandlerFactory()

	// Configure GitHub provider credentials
	configProvider.oauthCredentials[domain.ProviderTypeGitHub] = &driven.OAuthCredentials{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
	}

	// Register GitHub OAuth handler
	handlerFactory.handlers[domain.ProviderTypeGitHub] = newMockOAuthHandler()

	svc := NewOAuthService(OAuthServiceConfig{
		ConfigProvider:      configProvider,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installStore,
		OAuthHandlerFactory: handlerFactory,
	})

	// Test successful authorize
	resp, err := svc.Authorize(context.Background(), driving.AuthorizeRequest{
		ProviderType: domain.ProviderTypeGitHub,
	})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}

	if resp.AuthorizationURL == "" {
		t.Error("Authorize() returned empty AuthorizationURL")
	}
	if resp.State == "" {
		t.Error("Authorize() returned empty State")
	}
	if resp.ExpiresAt == "" {
		t.Error("Authorize() returned empty ExpiresAt")
	}

	// Verify state was stored
	if len(oauthStateStore.states) != 1 {
		t.Errorf("Expected 1 state stored, got %d", len(oauthStateStore.states))
	}
}

func TestOAuthService_Authorize_ProviderNotConfigured(t *testing.T) {
	configProvider := newMockConfigProvider()
	oauthStateStore := newMockOAuthStateStore()
	installStore := newMockInstallationStore()
	handlerFactory := newMockOAuthHandlerFactory()

	svc := NewOAuthService(OAuthServiceConfig{
		ConfigProvider:      configProvider,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installStore,
		OAuthHandlerFactory: handlerFactory,
	})

	// Test authorize with unconfigured provider
	_, err := svc.Authorize(context.Background(), driving.AuthorizeRequest{
		ProviderType: domain.ProviderTypeGitHub,
	})
	if err != driving.ErrOAuthProviderNotFound {
		t.Errorf("Authorize() error = %v, want ErrOAuthProviderNotFound", err)
	}
}

func TestOAuthService_Authorize_ProviderDisabled(t *testing.T) {
	configProvider := newMockConfigProvider()
	oauthStateStore := newMockOAuthStateStore()
	installStore := newMockInstallationStore()
	handlerFactory := newMockOAuthHandlerFactory()

	// Don't configure GitHub provider credentials (simulates disabled/unconfigured)
	// configProvider.oauthCredentials[domain.ProviderTypeGitHub] = nil

	svc := NewOAuthService(OAuthServiceConfig{
		ConfigProvider:      configProvider,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installStore,
		OAuthHandlerFactory: handlerFactory,
	})

	// Test authorize with disabled/unconfigured provider
	_, err := svc.Authorize(context.Background(), driving.AuthorizeRequest{
		ProviderType: domain.ProviderTypeGitHub,
	})
	if err != driving.ErrOAuthProviderNotFound {
		t.Errorf("Authorize() error = %v, want ErrOAuthProviderNotFound", err)
	}
}

func TestOAuthService_Callback_InvalidState(t *testing.T) {
	configProvider := newMockConfigProvider()
	oauthStateStore := newMockOAuthStateStore()
	installStore := newMockInstallationStore()
	handlerFactory := newMockOAuthHandlerFactory()

	svc := NewOAuthService(OAuthServiceConfig{
		ConfigProvider:      configProvider,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installStore,
		OAuthHandlerFactory: handlerFactory,
	})

	// Test callback with invalid state
	_, err := svc.Callback(context.Background(), driving.CallbackRequest{
		Code:  "test-code",
		State: "invalid-state",
	})
	if err != driving.ErrOAuthInvalidState {
		t.Errorf("Callback() error = %v, want ErrOAuthInvalidState", err)
	}
}

func TestOAuthService_Callback_ProviderError(t *testing.T) {
	configProvider := newMockConfigProvider()
	oauthStateStore := newMockOAuthStateStore()
	installStore := newMockInstallationStore()
	handlerFactory := newMockOAuthHandlerFactory()

	svc := NewOAuthService(OAuthServiceConfig{
		ConfigProvider:      configProvider,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installStore,
		OAuthHandlerFactory: handlerFactory,
	})

	// Test callback with error from provider
	_, err := svc.Callback(context.Background(), driving.CallbackRequest{
		State:            "some-state",
		Error:            "access_denied",
		ErrorDescription: "User denied access",
	})
	if err == nil {
		t.Error("Callback() expected error for provider error")
	}
	oauthErr, ok := err.(*driving.OAuthError)
	if !ok {
		t.Errorf("Callback() error type = %T, want *driving.OAuthError", err)
	} else if oauthErr.Code != "access_denied" {
		t.Errorf("Callback() error code = %s, want access_denied", oauthErr.Code)
	}
}

func TestGenerateRandomString(t *testing.T) {
	// Test that generateRandomString produces unique values
	s1, err := generateRandomString(32)
	if err != nil {
		t.Fatalf("generateRandomString() error = %v", err)
	}

	s2, err := generateRandomString(32)
	if err != nil {
		t.Fatalf("generateRandomString() error = %v", err)
	}

	if s1 == s2 {
		t.Error("generateRandomString() produced duplicate values")
	}

	if len(s1) != 32 {
		t.Errorf("generateRandomString(32) length = %d, want 32", len(s1))
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-code-verifier-12345678901234567890123456789012"
	challenge := generateCodeChallenge(verifier)

	// Challenge should be base64url encoded (no padding, URL-safe chars)
	if challenge == "" {
		t.Error("generateCodeChallenge() returned empty string")
	}

	// Same verifier should produce same challenge
	challenge2 := generateCodeChallenge(verifier)
	if challenge != challenge2 {
		t.Error("generateCodeChallenge() not deterministic")
	}

	// Different verifier should produce different challenge
	challenge3 := generateCodeChallenge("different-verifier")
	if challenge == challenge3 {
		t.Error("generateCodeChallenge() produced same challenge for different verifiers")
	}
}

func TestSplitScopes(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"repo", []string{"repo"}},
		{"repo read:user", []string{"repo", "read:user"}},
		{"repo,read:user", []string{"repo", "read:user"}},
		{"repo  read:user", []string{"repo", "read:user"}}, // multiple spaces
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitScopes(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitScopes(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitScopes(%q)[%d] = %s, want %s", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestProviderDisplayName(t *testing.T) {
	tests := []struct {
		providerType domain.ProviderType
		expected     string
	}{
		{domain.ProviderTypeGitHub, "GitHub"},
		{domain.ProviderTypeGitLab, "GitLab"},
		{domain.ProviderTypeSlack, "Slack"},
		{domain.ProviderTypeNotion, "Notion"},
		{domain.ProviderTypeConfluence, "Confluence"},
		{domain.ProviderTypeJira, "Jira"},
		{domain.ProviderTypeGoogleDrive, "Google Drive"},
		{domain.ProviderTypeGoogleDocs, "Google Docs"},
		{domain.ProviderTypeLinear, "Linear"},
		{domain.ProviderTypeDropbox, "Dropbox"},
		{domain.ProviderTypeS3, "Amazon S3"},
		{domain.ProviderType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			result := providerDisplayName(tt.providerType)
			if result != tt.expected {
				t.Errorf("providerDisplayName(%s) = %s, want %s", tt.providerType, result, tt.expected)
			}
		})
	}
}
