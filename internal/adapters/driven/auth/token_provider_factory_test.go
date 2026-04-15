package auth

import (
	"context"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// mockConnectionStore for testing
type mockConnectionStore struct {
	connections map[string]*domain.Connection
}

func newMockConnectionStore() *mockConnectionStore {
	return &mockConnectionStore{
		connections: make(map[string]*domain.Connection),
	}
}

func (m *mockConnectionStore) Save(ctx context.Context, conn *domain.Connection) error {
	m.connections[conn.ID] = conn
	return nil
}

func (m *mockConnectionStore) Get(ctx context.Context, id string) (*domain.Connection, error) {
	conn, ok := m.connections[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return conn, nil
}

func (m *mockConnectionStore) List(ctx context.Context) ([]*domain.ConnectionSummary, error) {
	return nil, nil
}

func (m *mockConnectionStore) Delete(ctx context.Context, id string) error {
	delete(m.connections, id)
	return nil
}

func (m *mockConnectionStore) GetByPlatform(ctx context.Context, platform domain.PlatformType) ([]*domain.ConnectionSummary, error) {
	return nil, nil
}

func (m *mockConnectionStore) GetByAccountID(ctx context.Context, platform domain.PlatformType, accountID string) (*domain.Connection, error) {
	return nil, nil
}

func (m *mockConnectionStore) UpdateSecrets(ctx context.Context, id string, secrets *domain.ConnectionSecrets, expiry *time.Time) error {
	conn, ok := m.connections[id]
	if !ok {
		return domain.ErrNotFound
	}
	conn.Secrets = secrets
	conn.OAuthExpiry = expiry
	return nil
}

func (m *mockConnectionStore) UpdateLastUsed(ctx context.Context, id string) error {
	return nil
}

func TestNewTokenProviderFactory(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	if factory.connectionStore != connStore {
		t.Error("expected connection store to be set")
	}
	if factory.refreshers == nil {
		t.Error("expected refreshers map to be initialized")
	}
}

func TestRegisterRefresher_WithPlatformType(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	// Mock refresher function
	mockRefresher := func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		return &driven.OAuthToken{
			AccessToken:  "new_token",
			RefreshToken: "new_refresh",
			ExpiresIn:    3600,
		}, nil
	}

	// Register refresher for GitHub platform
	factory.RegisterRefresher(domain.PlatformGitHub, mockRefresher)

	// Verify it was registered
	if factory.refreshers[domain.PlatformGitHub] == nil {
		t.Error("expected GitHub refresher to be registered")
	}

	// Register refresher for LocalFS platform
	factory.RegisterRefresher(domain.PlatformLocalFS, mockRefresher)

	// Verify it was registered
	if factory.refreshers[domain.PlatformLocalFS] == nil {
		t.Error("expected LocalFS refresher to be registered")
	}

	// Verify we have exactly 2 refreshers
	if len(factory.refreshers) != 2 {
		t.Errorf("expected 2 refreshers, got %d", len(factory.refreshers))
	}
}

func TestCreateFromConnection_OAuth2(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	// Register refresher for GitHub
	mockRefresher := func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		return &driven.OAuthToken{
			AccessToken:  "refreshed_token",
			RefreshToken: "new_refresh",
			ExpiresIn:    3600,
		}, nil
	}
	factory.RegisterRefresher(domain.PlatformGitHub, mockRefresher)

	expiry := time.Now().Add(1 * time.Hour)
	conn := &domain.Connection{
		ID:           "conn-1",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		OAuthExpiry:  &expiry,
		Secrets: &domain.ConnectionSecrets{
			AccessToken:  "access_token",
			RefreshToken: "refresh_token",
		},
	}

	provider, err := factory.CreateFromConnection(context.Background(), conn)
	if err != nil {
		t.Fatalf("CreateFromConnection() error = %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	// Verify it's an OAuth provider
	if provider.AuthMethod() != domain.AuthMethodOAuth2 {
		t.Errorf("expected AuthMethod OAuth2, got %s", provider.AuthMethod())
	}

	// Verify we can get a token
	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != "access_token" {
		t.Errorf("expected token 'access_token', got %s", token)
	}
}

func TestCreateFromConnection_APIKey(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	conn := &domain.Connection{
		ID:           "conn-2",
		Platform:     domain.PlatformLocalFS,
		ProviderType: domain.ProviderTypeLocalFS,
		AuthMethod:   domain.AuthMethodAPIKey,
		Secrets: &domain.ConnectionSecrets{
			APIKey: "api-key-123",
		},
	}

	provider, err := factory.CreateFromConnection(context.Background(), conn)
	if err != nil {
		t.Fatalf("CreateFromConnection() error = %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	// Verify it's an API Key provider
	if provider.AuthMethod() != domain.AuthMethodAPIKey {
		t.Errorf("expected AuthMethod APIKey, got %s", provider.AuthMethod())
	}

	// Verify we can get the token
	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != "api-key-123" {
		t.Errorf("expected token 'api-key-123', got %s", token)
	}
}

func TestCreateFromConnection_PAT(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	conn := &domain.Connection{
		ID:           "conn-3",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodPAT,
		Secrets: &domain.ConnectionSecrets{
			APIKey: "ghp_pat_token",
		},
	}

	provider, err := factory.CreateFromConnection(context.Background(), conn)
	if err != nil {
		t.Fatalf("CreateFromConnection() error = %v", err)
	}

	if provider.AuthMethod() != domain.AuthMethodPAT {
		t.Errorf("expected AuthMethod PAT, got %s", provider.AuthMethod())
	}

	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != "ghp_pat_token" {
		t.Errorf("expected token 'ghp_pat_token', got %s", token)
	}
}

func TestCreateFromConnection_ServiceAccount(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	serviceAccountJSON := `{"type": "service_account", "project_id": "test"}`
	conn := &domain.Connection{
		ID:           "conn-4",
		Platform:     domain.PlatformType("google"),
		ProviderType: domain.ProviderType("google_drive"),
		AuthMethod:   domain.AuthMethodServiceAccount,
		Secrets: &domain.ConnectionSecrets{
			ServiceAccountJSON: serviceAccountJSON,
		},
	}

	provider, err := factory.CreateFromConnection(context.Background(), conn)
	if err != nil {
		t.Fatalf("CreateFromConnection() error = %v", err)
	}

	if provider.AuthMethod() != domain.AuthMethodServiceAccount {
		t.Errorf("expected AuthMethod ServiceAccount, got %s", provider.AuthMethod())
	}

	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != serviceAccountJSON {
		t.Errorf("expected service account JSON, got %s", token)
	}
}

func TestCreateFromConnection_NoSecrets(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	conn := &domain.Connection{
		ID:           "conn-5",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		Secrets:      nil, // No secrets
	}

	_, err := factory.CreateFromConnection(context.Background(), conn)
	if err == nil {
		t.Error("expected error for connection with no secrets")
	}
}

func TestCreateFromConnection_UnsupportedAuthMethod(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	conn := &domain.Connection{
		ID:           "conn-6",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethod("unknown"),
		Secrets: &domain.ConnectionSecrets{
			AccessToken: "token",
		},
	}

	_, err := factory.CreateFromConnection(context.Background(), conn)
	if err == nil {
		t.Error("expected error for unsupported auth method")
	}
}

func TestCreate_ConnectionNotFound(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	_, err := factory.Create(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent connection")
	}
}

func TestCreate_Success(t *testing.T) {
	connStore := newMockConnectionStore()
	factory := NewTokenProviderFactory(connStore)

	// Save a connection
	conn := &domain.Connection{
		ID:           "conn-7",
		Platform:     domain.PlatformLocalFS,
		ProviderType: domain.ProviderTypeLocalFS,
		AuthMethod:   domain.AuthMethodAPIKey,
		Secrets: &domain.ConnectionSecrets{
			APIKey: "test-key",
		},
	}
	_ = connStore.Save(context.Background(), conn)

	provider, err := factory.Create(context.Background(), "conn-7")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestStaticTokenProvider_GetAccessToken(t *testing.T) {
	provider := NewStaticTokenProvider("static-token", domain.AuthMethodAPIKey)

	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != "static-token" {
		t.Errorf("expected token 'static-token', got %s", token)
	}
}

func TestStaticTokenProvider_GetCredentials(t *testing.T) {
	provider := NewStaticTokenProvider("api-key-value", domain.AuthMethodAPIKey)

	creds, err := provider.GetCredentials(context.Background())
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}
	if creds.AuthMethod != domain.AuthMethodAPIKey {
		t.Errorf("expected AuthMethod APIKey, got %s", creds.AuthMethod)
	}
	if creds.APIKey != "api-key-value" {
		t.Errorf("expected APIKey 'api-key-value', got %s", creds.APIKey)
	}
}

func TestStaticTokenProvider_IsValid(t *testing.T) {
	provider := NewStaticTokenProvider("token", domain.AuthMethodAPIKey)

	if !provider.IsValid(context.Background()) {
		t.Error("expected static token provider to always be valid")
	}
}

func TestOAuthTokenProvider_GetAccessToken(t *testing.T) {
	connStore := newMockConnectionStore()
	expiry := time.Now().Add(1 * time.Hour)

	provider := NewOAuthTokenProvider(
		"conn-1",
		"access_token",
		"refresh_token",
		&expiry,
		nil,
		connStore,
	)

	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token != "access_token" {
		t.Errorf("expected token 'access_token', got %s", token)
	}
}

func TestOAuthTokenProvider_IsValid_WithRefresher(t *testing.T) {
	connStore := newMockConnectionStore()
	expiry := time.Now().Add(-1 * time.Hour) // Expired

	mockRefresher := func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		return &driven.OAuthToken{
			AccessToken:  "new_token",
			RefreshToken: "new_refresh",
			ExpiresIn:    3600,
		}, nil
	}

	provider := NewOAuthTokenProvider(
		"conn-1",
		"access_token",
		"refresh_token",
		&expiry,
		mockRefresher,
		connStore,
	)

	// Even with expired token, should be valid because we have a refresher
	if !provider.IsValid(context.Background()) {
		t.Error("expected provider to be valid with refresher")
	}
}

func TestOAuthTokenProvider_IsValid_WithoutRefresher(t *testing.T) {
	connStore := newMockConnectionStore()
	expiry := time.Now().Add(-1 * time.Hour) // Expired

	provider := NewOAuthTokenProvider(
		"conn-1",
		"access_token",
		"", // No refresh token
		&expiry,
		nil, // No refresher
		connStore,
	)

	// Without refresher and expired token, should not be valid
	if provider.IsValid(context.Background()) {
		t.Error("expected provider to be invalid without refresher and expired token")
	}
}

func TestOAuthTokenProvider_Refresh(t *testing.T) {
	connStore := newMockConnectionStore()

	// Save connection to store
	conn := &domain.Connection{
		ID:       "conn-1",
		Platform: domain.PlatformGitHub,
		Secrets: &domain.ConnectionSecrets{
			AccessToken:  "old_token",
			RefreshToken: "refresh_token",
		},
	}
	_ = connStore.Save(context.Background(), conn)

	expiry := time.Now().Add(2 * time.Minute) // Needs refresh (within 5 minutes)

	refreshCalled := false
	mockRefresher := func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		refreshCalled = true
		if refreshToken != "refresh_token" {
			t.Errorf("expected refresh token 'refresh_token', got %s", refreshToken)
		}
		return &driven.OAuthToken{
			AccessToken:  "new_token",
			RefreshToken: "new_refresh",
			ExpiresIn:    3600,
		}, nil
	}

	provider := NewOAuthTokenProvider(
		"conn-1",
		"old_token",
		"refresh_token",
		&expiry,
		mockRefresher,
		connStore,
	)

	token, err := provider.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}

	if !refreshCalled {
		t.Error("expected refresher to be called")
	}

	if token != "new_token" {
		t.Errorf("expected token 'new_token', got %s", token)
	}

	// Verify connection was updated in store
	updatedConn, _ := connStore.Get(context.Background(), "conn-1")
	if updatedConn.Secrets.AccessToken != "new_token" {
		t.Errorf("expected connection to be updated with new token")
	}
}
