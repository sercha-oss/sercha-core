package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// mockOAuthClientStore implements driven.OAuthClientStore for testing
type mockOAuthClientStore struct {
	clients map[string]*domain.OAuthClient
}

func newMockOAuthClientStore() *mockOAuthClientStore {
	return &mockOAuthClientStore{
		clients: make(map[string]*domain.OAuthClient),
	}
}

func (m *mockOAuthClientStore) Save(ctx context.Context, client *domain.OAuthClient) error {
	m.clients[client.ID] = client
	return nil
}

func (m *mockOAuthClientStore) Get(ctx context.Context, clientID string) (*domain.OAuthClient, error) {
	client, ok := m.clients[clientID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return client, nil
}

func (m *mockOAuthClientStore) Delete(ctx context.Context, clientID string) error {
	delete(m.clients, clientID)
	return nil
}

func (m *mockOAuthClientStore) List(ctx context.Context) ([]*domain.OAuthClient, error) {
	clients := make([]*domain.OAuthClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	return clients, nil
}

// mockAuthorizationCodeStore implements driven.AuthorizationCodeStore for testing
type mockAuthorizationCodeStore struct {
	codes map[string]*domain.AuthorizationCode
}

func newMockAuthorizationCodeStore() *mockAuthorizationCodeStore {
	return &mockAuthorizationCodeStore{
		codes: make(map[string]*domain.AuthorizationCode),
	}
}

func (m *mockAuthorizationCodeStore) Save(ctx context.Context, code *domain.AuthorizationCode) error {
	m.codes[code.Code] = code
	return nil
}

func (m *mockAuthorizationCodeStore) GetAndMarkUsed(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	authCode, ok := m.codes[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	// Return a copy of the code before marking as used
	codeCopy := *authCode
	// Mark the stored version as used
	authCode.Used = true
	return &codeCopy, nil
}

func (m *mockAuthorizationCodeStore) Cleanup(ctx context.Context) error {
	now := time.Now()
	for k, v := range m.codes {
		if now.After(v.ExpiresAt) {
			delete(m.codes, k)
		}
	}
	return nil
}

// mockOAuthTokenStore implements driven.OAuthTokenStore for testing
type mockOAuthTokenStore struct {
	accessTokens  map[string]*domain.OAuthAccessToken
	refreshTokens map[string]*domain.OAuthRefreshToken
}

func newMockOAuthTokenStore() *mockOAuthTokenStore {
	return &mockOAuthTokenStore{
		accessTokens:  make(map[string]*domain.OAuthAccessToken),
		refreshTokens: make(map[string]*domain.OAuthRefreshToken),
	}
}

func (m *mockOAuthTokenStore) SaveAccessToken(ctx context.Context, token *domain.OAuthAccessToken) error {
	m.accessTokens[token.ID] = token
	return nil
}

func (m *mockOAuthTokenStore) GetAccessToken(ctx context.Context, tokenID string) (*domain.OAuthAccessToken, error) {
	token, ok := m.accessTokens[tokenID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return token, nil
}

func (m *mockOAuthTokenStore) RevokeAccessToken(ctx context.Context, tokenID string) error {
	token, ok := m.accessTokens[tokenID]
	if !ok {
		return domain.ErrNotFound
	}
	token.Revoked = true
	return nil
}

func (m *mockOAuthTokenStore) SaveRefreshToken(ctx context.Context, token *domain.OAuthRefreshToken) error {
	m.refreshTokens[token.ID] = token
	return nil
}

func (m *mockOAuthTokenStore) GetRefreshToken(ctx context.Context, tokenID string) (*domain.OAuthRefreshToken, error) {
	token, ok := m.refreshTokens[tokenID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return token, nil
}

func (m *mockOAuthTokenStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	token, ok := m.refreshTokens[tokenID]
	if !ok {
		return domain.ErrNotFound
	}
	token.Revoked = true
	return nil
}

func (m *mockOAuthTokenStore) RotateRefreshToken(ctx context.Context, oldTokenID string, newTokenID string) error {
	token, ok := m.refreshTokens[oldTokenID]
	if !ok {
		return domain.ErrNotFound
	}
	token.RotatedTo = newTokenID
	return nil
}

func (m *mockOAuthTokenStore) RevokeAllForClient(ctx context.Context, clientID string) error {
	for _, token := range m.accessTokens {
		if token.ClientID == clientID {
			token.Revoked = true
		}
	}
	for _, token := range m.refreshTokens {
		if token.ClientID == clientID {
			token.Revoked = true
		}
	}
	return nil
}

func (m *mockOAuthTokenStore) Cleanup(ctx context.Context) error {
	now := time.Now()
	for k, v := range m.accessTokens {
		if now.After(v.ExpiresAt) {
			delete(m.accessTokens, k)
		}
	}
	for k, v := range m.refreshTokens {
		if now.After(v.ExpiresAt) {
			delete(m.refreshTokens, k)
		}
	}
	return nil
}

// Test helper to create a service with mocked stores
func createTestService() (*oauthServerService, *mockOAuthClientStore, *mockAuthorizationCodeStore, *mockOAuthTokenStore) {
	clientStore := newMockOAuthClientStore()
	codeStore := newMockAuthorizationCodeStore()
	tokenStore := newMockOAuthTokenStore()

	svc := NewOAuthServerService(OAuthServerServiceConfig{
		ClientStore:  clientStore,
		CodeStore:    codeStore,
		TokenStore:   tokenStore,
		JWTSecret:    "test-jwt-secret-key-for-testing",
		MCPServerURL: "http://localhost:3000",
	}).(*oauthServerService)

	return svc, clientStore, codeStore, tokenStore
}

// TestRegisterClient_Success tests successful client registration with defaults
func TestRegisterClient_Success(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:         "Test Client",
		RedirectURIs: []string{"http://localhost:8080/callback"},
	}

	resp, err := svc.RegisterClient(context.Background(), req)
	if err != nil {
		t.Fatalf("RegisterClient() error = %v", err)
	}

	// Verify response
	if resp.ClientID == "" {
		t.Error("expected non-empty ClientID")
	}
	if resp.ClientSecret != "" {
		t.Error("expected empty ClientSecret for public client")
	}
	if resp.Name != "Test Client" {
		t.Errorf("Name = %s, want Test Client", resp.Name)
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "http://localhost:8080/callback" {
		t.Errorf("RedirectURIs = %v, want [http://localhost:8080/callback]", resp.RedirectURIs)
	}
	if len(resp.GrantTypes) != 1 || resp.GrantTypes[0] != domain.GrantTypeAuthorizationCode {
		t.Errorf("GrantTypes = %v, want [authorization_code]", resp.GrantTypes)
	}
	if len(resp.ResponseTypes) != 1 || resp.ResponseTypes[0] != "code" {
		t.Errorf("ResponseTypes = %v, want [code]", resp.ResponseTypes)
	}
	if len(resp.Scopes) != 3 {
		t.Errorf("Scopes length = %d, want 3 (default MCP scopes)", len(resp.Scopes))
	}
	if resp.ApplicationType != "native" {
		t.Errorf("ApplicationType = %s, want native", resp.ApplicationType)
	}
	if resp.TokenEndpointAuthMethod != "none" {
		t.Errorf("TokenEndpointAuthMethod = %s, want none", resp.TokenEndpointAuthMethod)
	}

	// Verify client was saved
	client, err := clientStore.Get(context.Background(), resp.ClientID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !client.Active {
		t.Error("expected client to be active")
	}
}

// TestRegisterClient_ConfidentialClient tests registering a confidential client
func TestRegisterClient_ConfidentialClient(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:                    "Confidential Client",
		RedirectURIs:            []string{"https://app.example.com/callback"},
		TokenEndpointAuthMethod: "client_secret_post",
		ApplicationType:         "web",
	}

	resp, err := svc.RegisterClient(context.Background(), req)
	if err != nil {
		t.Fatalf("RegisterClient() error = %v", err)
	}

	// Verify response includes client secret
	if resp.ClientSecret == "" {
		t.Error("expected non-empty ClientSecret for confidential client")
	}
	if resp.TokenEndpointAuthMethod != "client_secret_post" {
		t.Errorf("TokenEndpointAuthMethod = %s, want client_secret_post", resp.TokenEndpointAuthMethod)
	}
	if resp.ApplicationType != "web" {
		t.Errorf("ApplicationType = %s, want web", resp.ApplicationType)
	}

	// Verify client secret is hashed in store
	client, _ := clientStore.Get(context.Background(), resp.ClientID)
	if client.SecretHash == "" {
		t.Error("expected SecretHash to be set")
	}
	if client.SecretHash == resp.ClientSecret {
		t.Error("SecretHash should not be plaintext")
	}

	// Verify bcrypt hash is valid
	err = bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(resp.ClientSecret))
	if err != nil {
		t.Error("client secret hash verification failed")
	}
}

// TestRegisterClient_MissingName tests error when name is missing
func TestRegisterClient_MissingName(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:         "",
		RedirectURIs: []string{"http://localhost:8080/callback"},
	}

	_, err := svc.RegisterClient(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "client_name") {
		t.Errorf("error = %v, want error mentioning client_name", err)
	}
}

// TestRegisterClient_MissingRedirectURIs tests error when redirect URIs are missing
func TestRegisterClient_MissingRedirectURIs(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:         "Test Client",
		RedirectURIs: []string{},
	}

	_, err := svc.RegisterClient(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing redirect URIs")
	}
	if !strings.Contains(err.Error(), "redirect_uris") {
		t.Errorf("error = %v, want error mentioning redirect_uris", err)
	}
}

// TestRegisterClient_InvalidScope tests error when an invalid scope is requested
func TestRegisterClient_InvalidScope(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:         "Test Client",
		RedirectURIs: []string{"http://localhost:8080/callback"},
		Scopes:       []string{domain.ScopeMCPSearch, "invalid:scope"},
	}

	_, err := svc.RegisterClient(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "scope") {
		t.Errorf("error = %v, want error mentioning scope", err)
	}
}

// TestRegisterClient_InvalidApplicationType tests error when application type is invalid
func TestRegisterClient_InvalidApplicationType(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.ClientRegistrationRequest{
		Name:            "Test Client",
		RedirectURIs:    []string{"http://localhost:8080/callback"},
		ApplicationType: "invalid",
	}

	_, err := svc.RegisterClient(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid application type")
	}
	if !strings.Contains(err.Error(), "application_type") {
		t.Errorf("error = %v, want error mentioning application_type", err)
	}
}

// TestAuthorize_Success tests successful authorization flow
func TestAuthorize_Success(t *testing.T) {
	svc, clientStore, codeStore, _ := createTestService()

	// Register a client
	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		Name:                    "Test Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		ResponseTypes:           []string{"code"},
		Scopes:                  domain.DefaultMCPScopes,
		ApplicationType:         "native",
		TokenEndpointAuthMethod: "none",
		Active:                  true,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client-id",
		RedirectURI:         "http://localhost:8080/callback",
		Scope:               "mcp:search mcp:documents:read",
		State:               "random-state",
		CodeChallenge:       "test-challenge",
		CodeChallengeMethod: "S256",
		Resource:            "http://localhost:3000",
	}

	code, err := svc.Authorize(context.Background(), "user-123", req)
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}

	if code == "" {
		t.Error("expected non-empty authorization code")
	}

	// Verify code was saved
	authCode, ok := codeStore.codes[code]
	if !ok || authCode == nil {
		t.Fatal("authorization code not found in store")
	}
	if authCode.ClientID != "test-client-id" {
		t.Errorf("ClientID = %s, want test-client-id", authCode.ClientID)
	}
	if authCode.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", authCode.UserID)
	}
	if authCode.CodeChallenge != "test-challenge" {
		t.Errorf("CodeChallenge = %s, want test-challenge", authCode.CodeChallenge)
	}
}

// TestAuthorize_UnsupportedResponseType tests error for unsupported response type
func TestAuthorize_UnsupportedResponseType(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.AuthorizeRequest{
		ResponseType:        "token",
		ClientID:            "test-client-id",
		RedirectURI:         "http://localhost:8080/callback",
		CodeChallengeMethod: "S256",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for unsupported response type")
	}
	if !strings.Contains(err.Error(), "response_type") {
		t.Errorf("error = %v, want error mentioning response_type", err)
	}
}

// TestAuthorize_MissingPKCE tests error when PKCE is missing
func TestAuthorize_MissingPKCE(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client-id",
		RedirectURI:         "http://localhost:8080/callback",
		CodeChallengeMethod: "S256",
		CodeChallenge:       "", // Missing
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for missing PKCE")
	}
	if !strings.Contains(err.Error(), "code_challenge") {
		t.Errorf("error = %v, want error mentioning code_challenge", err)
	}
}

// TestAuthorize_InvalidPKCEMethod tests error when PKCE method is invalid
func TestAuthorize_InvalidPKCEMethod(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client-id",
		RedirectURI:         "http://localhost:8080/callback",
		CodeChallengeMethod: "plain",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for invalid PKCE method")
	}
	if !strings.Contains(err.Error(), "code_challenge_method") {
		t.Errorf("error = %v, want error mentioning code_challenge_method", err)
	}
}

// TestAuthorize_ClientNotFound tests error when client doesn't exist
func TestAuthorize_ClientNotFound(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "nonexistent-client",
		RedirectURI:         "http://localhost:8080/callback",
		CodeChallengeMethod: "S256",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for nonexistent client")
	}
	if !strings.Contains(err.Error(), "client") {
		t.Errorf("error = %v, want error mentioning client", err)
	}
}

// TestAuthorize_InactiveClient tests error when client is inactive
func TestAuthorize_InactiveClient(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	// Register inactive client
	client := &domain.OAuthClient{
		ID:                      "inactive-client",
		Name:                    "Inactive Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		ResponseTypes:           []string{"code"},
		Scopes:                  domain.DefaultMCPScopes,
		ApplicationType:         "native",
		TokenEndpointAuthMethod: "none",
		Active:                  false, // Inactive
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "inactive-client",
		RedirectURI:         "http://localhost:8080/callback",
		CodeChallengeMethod: "S256",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for inactive client")
	}
	if !strings.Contains(err.Error(), "inactive") {
		t.Errorf("error = %v, want error mentioning inactive", err)
	}
}

// TestAuthorize_InvalidRedirectURI tests error when redirect URI is not registered
func TestAuthorize_InvalidRedirectURI(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		Name:                    "Test Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		ResponseTypes:           []string{"code"},
		Scopes:                  domain.DefaultMCPScopes,
		ApplicationType:         "native",
		TokenEndpointAuthMethod: "none",
		Active:                  true,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client-id",
		RedirectURI:         "http://malicious.com/callback", // Not registered
		CodeChallengeMethod: "S256",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for invalid redirect URI")
	}
	if !strings.Contains(err.Error(), "redirect_uri") {
		t.Errorf("error = %v, want error mentioning redirect_uri", err)
	}
}

// TestAuthorize_InvalidScope tests error when requested scope is not allowed
func TestAuthorize_InvalidScope(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		Name:                    "Test Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		ResponseTypes:           []string{"code"},
		Scopes:                  []string{domain.ScopeMCPSearch}, // Only search scope
		ApplicationType:         "native",
		TokenEndpointAuthMethod: "none",
		Active:                  true,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	req := domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client-id",
		RedirectURI:         "http://localhost:8080/callback",
		Scope:               "mcp:search mcp:documents:read", // Requesting disallowed scope
		CodeChallengeMethod: "S256",
		CodeChallenge:       "test-challenge",
	}

	_, err := svc.Authorize(context.Background(), "user-123", req)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "scope") {
		t.Errorf("error = %v, want error mentioning scope", err)
	}
}

// TestToken_AuthorizationCode_Success tests successful token exchange
func TestToken_AuthorizationCode_Success(t *testing.T) {
	svc, clientStore, codeStore, tokenStore := createTestService()

	// Register client
	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		Name:                    "Test Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		ResponseTypes:           []string{"code"},
		Scopes:                  domain.DefaultMCPScopes,
		ApplicationType:         "native",
		TokenEndpointAuthMethod: "none",
		Active:                  true,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	// Create code verifier and challenge
	verifier := "test-code-verifier-1234567890123456789012345678901234567890"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// Save authorization code
	authCode := &domain.AuthorizationCode{
		Code:          "test-auth-code",
		ClientID:      "test-client-id",
		UserID:        "user-123",
		RedirectURI:   "http://localhost:8080/callback",
		Scopes:        []string{domain.ScopeMCPSearch},
		CodeChallenge: challenge,
		Resource:      "http://localhost:3000",
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Used:          false,
		CreatedAt:     time.Now(),
	}
	_ = codeStore.Save(context.Background(), authCode)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "test-auth-code",
		RedirectURI:  "http://localhost:8080/callback",
		ClientID:     "test-client-id",
		CodeVerifier: verifier,
	}

	resp, err := svc.Token(context.Background(), req)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}

	// Verify response
	if resp.AccessToken == "" {
		t.Error("expected non-empty AccessToken")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("TokenType = %s, want Bearer", resp.TokenType)
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty RefreshToken")
	}
	if resp.ExpiresIn != int64(domain.AccessTokenTTL.Seconds()) {
		t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, int64(domain.AccessTokenTTL.Seconds()))
	}

	// Verify code was marked as used
	usedCode := codeStore.codes["test-auth-code"]
	if !usedCode.Used {
		t.Error("expected authorization code to be marked as used")
	}

	// Verify tokens were saved
	if len(tokenStore.accessTokens) == 0 {
		t.Error("expected access token to be saved")
	}
	if len(tokenStore.refreshTokens) == 0 {
		t.Error("expected refresh token to be saved")
	}
}

// TestToken_AuthorizationCode_InvalidCode tests error for invalid code
func TestToken_AuthorizationCode_InvalidCode(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "invalid-code",
		RedirectURI:  "http://localhost:8080/callback",
		ClientID:     "test-client-id",
		CodeVerifier: "test-verifier",
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid code")
	}
}

// TestToken_AuthorizationCode_ExpiredCode tests error for expired code
func TestToken_AuthorizationCode_ExpiredCode(t *testing.T) {
	svc, clientStore, codeStore, _ := createTestService()

	// Register client
	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		Name:                    "Test Client",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client)

	// Save expired authorization code
	authCode := &domain.AuthorizationCode{
		Code:          "expired-code",
		ClientID:      "test-client-id",
		UserID:        "user-123",
		RedirectURI:   "http://localhost:8080/callback",
		Scopes:        []string{domain.ScopeMCPSearch},
		CodeChallenge: "test-challenge",
		ExpiresAt:     time.Now().Add(-1 * time.Minute), // Expired
		Used:          false,
		CreatedAt:     time.Now(),
	}
	_ = codeStore.Save(context.Background(), authCode)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "expired-code",
		RedirectURI:  "http://localhost:8080/callback",
		ClientID:     "test-client-id",
		CodeVerifier: "test-verifier",
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for expired code")
	}
	if err != domain.ErrExpiredCode {
		t.Errorf("error = %v, want ErrExpiredCode", err)
	}
}

// TestToken_AuthorizationCode_ClientMismatch tests error when client ID doesn't match
func TestToken_AuthorizationCode_ClientMismatch(t *testing.T) {
	svc, clientStore, codeStore, _ := createTestService()

	// Register clients
	client1 := &domain.OAuthClient{
		ID:                      "client-1",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	client2 := &domain.OAuthClient{
		ID:                      "client-2",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client1)
	_ = clientStore.Save(context.Background(), client2)

	// Save authorization code for client-1
	authCode := &domain.AuthorizationCode{
		Code:          "auth-code",
		ClientID:      "client-1",
		UserID:        "user-123",
		RedirectURI:   "http://localhost:8080/callback",
		Scopes:        []string{domain.ScopeMCPSearch},
		CodeChallenge: "test-challenge",
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Used:          false,
		CreatedAt:     time.Now(),
	}
	_ = codeStore.Save(context.Background(), authCode)

	// Try to use code with client-2
	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "auth-code",
		RedirectURI:  "http://localhost:8080/callback",
		ClientID:     "client-2", // Wrong client
		CodeVerifier: "test-verifier",
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for client mismatch")
	}
	if !strings.Contains(err.Error(), "client") {
		t.Errorf("error = %v, want error mentioning client", err)
	}
}

// TestToken_AuthorizationCode_RedirectMismatch tests error when redirect URI doesn't match
func TestToken_AuthorizationCode_RedirectMismatch(t *testing.T) {
	svc, clientStore, codeStore, _ := createTestService()

	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		RedirectURIs:            []string{"http://localhost:8080/callback", "http://localhost:8080/other"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client)

	authCode := &domain.AuthorizationCode{
		Code:          "auth-code",
		ClientID:      "test-client-id",
		UserID:        "user-123",
		RedirectURI:   "http://localhost:8080/callback",
		Scopes:        []string{domain.ScopeMCPSearch},
		CodeChallenge: "test-challenge",
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Used:          false,
		CreatedAt:     time.Now(),
	}
	_ = codeStore.Save(context.Background(), authCode)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "auth-code",
		RedirectURI:  "http://localhost:8080/other", // Wrong redirect URI
		ClientID:     "test-client-id",
		CodeVerifier: "test-verifier",
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for redirect URI mismatch")
	}
	if !strings.Contains(err.Error(), "redirect_uri") {
		t.Errorf("error = %v, want error mentioning redirect_uri", err)
	}
}

// TestToken_AuthorizationCode_InvalidPKCE tests error when PKCE verifier is invalid
func TestToken_AuthorizationCode_InvalidPKCE(t *testing.T) {
	svc, clientStore, codeStore, _ := createTestService()

	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		RedirectURIs:            []string{"http://localhost:8080/callback"},
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client)

	// Create valid code challenge
	verifier := "correct-verifier"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	authCode := &domain.AuthorizationCode{
		Code:          "auth-code",
		ClientID:      "test-client-id",
		UserID:        "user-123",
		RedirectURI:   "http://localhost:8080/callback",
		Scopes:        []string{domain.ScopeMCPSearch},
		CodeChallenge: challenge,
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Used:          false,
		CreatedAt:     time.Now(),
	}
	_ = codeStore.Save(context.Background(), authCode)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeAuthorizationCode,
		Code:         "auth-code",
		RedirectURI:  "http://localhost:8080/callback",
		ClientID:     "test-client-id",
		CodeVerifier: "wrong-verifier", // Wrong verifier
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid PKCE verifier")
	}
	if !strings.Contains(err.Error(), "code_verifier") {
		t.Errorf("error = %v, want error mentioning code_verifier", err)
	}
}

// TestToken_RefreshToken_Success tests successful token refresh with rotation
func TestToken_RefreshToken_Success(t *testing.T) {
	svc, clientStore, _, tokenStore := createTestService()

	// Register client
	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client)

	// Save refresh token
	refreshToken := &domain.OAuthRefreshToken{
		ID:            "old-refresh-token",
		AccessTokenID: "old-access-token",
		ClientID:      "test-client-id",
		UserID:        "user-123",
		Scopes:        []string{domain.ScopeMCPSearch},
		Audience:      "http://localhost:3000",
		ExpiresAt:     time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		Revoked:       false,
		RotatedTo:     "",
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), refreshToken)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeRefreshToken,
		RefreshToken: "old-refresh-token",
		ClientID:     "test-client-id",
	}

	resp, err := svc.Token(context.Background(), req)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}

	// Verify response
	if resp.AccessToken == "" {
		t.Error("expected non-empty AccessToken")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty RefreshToken")
	}
	if resp.RefreshToken == "old-refresh-token" {
		t.Error("expected new refresh token (rotation)")
	}

	// Verify old refresh token was rotated
	oldToken, _ := tokenStore.GetRefreshToken(context.Background(), "old-refresh-token")
	if oldToken.RotatedTo == "" {
		t.Error("expected old refresh token to be rotated")
	}
	if oldToken.RotatedTo != resp.RefreshToken {
		t.Errorf("RotatedTo = %s, want %s", oldToken.RotatedTo, resp.RefreshToken)
	}
}

// TestToken_RefreshToken_Invalid tests error for invalid refresh token
func TestToken_RefreshToken_Invalid(t *testing.T) {
	svc, clientStore, _, tokenStore := createTestService()

	client := &domain.OAuthClient{
		ID:                      "test-client-id",
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client)

	// Test expired token
	expiredToken := &domain.OAuthRefreshToken{
		ID:        "expired-token",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		Revoked:   false,
		RotatedTo: "",
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), expiredToken)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeRefreshToken,
		RefreshToken: "expired-token",
		ClientID:     "test-client-id",
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Error("expected error for expired refresh token")
	}

	// Test revoked token
	revokedToken := &domain.OAuthRefreshToken{
		ID:        "revoked-token",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Revoked:   true, // Revoked
		RotatedTo: "",
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), revokedToken)

	req.RefreshToken = "revoked-token"
	_, err = svc.Token(context.Background(), req)
	if err == nil {
		t.Error("expected error for revoked refresh token")
	}

	// Test rotated token
	rotatedToken := &domain.OAuthRefreshToken{
		ID:        "rotated-token",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Revoked:   false,
		RotatedTo: "new-token", // Rotated
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), rotatedToken)

	req.RefreshToken = "rotated-token"
	_, err = svc.Token(context.Background(), req)
	if err == nil {
		t.Error("expected error for rotated refresh token")
	}
}

// TestToken_RefreshToken_ClientMismatch tests error when client ID doesn't match
func TestToken_RefreshToken_ClientMismatch(t *testing.T) {
	svc, clientStore, _, tokenStore := createTestService()

	client1 := &domain.OAuthClient{
		ID:                      "client-1",
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	client2 := &domain.OAuthClient{
		ID:                      "client-2",
		GrantTypes:              []domain.OAuthGrantType{domain.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: "none",
		Active:                  true,
	}
	_ = clientStore.Save(context.Background(), client1)
	_ = clientStore.Save(context.Background(), client2)

	refreshToken := &domain.OAuthRefreshToken{
		ID:        "refresh-token",
		ClientID:  "client-1",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Revoked:   false,
		RotatedTo: "",
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), refreshToken)

	req := domain.TokenRequest{
		GrantType:    domain.GrantTypeRefreshToken,
		RefreshToken: "refresh-token",
		ClientID:     "client-2", // Wrong client
	}

	_, err := svc.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for client mismatch")
	}
	if !strings.Contains(err.Error(), "client") {
		t.Errorf("error = %v, want error mentioning client", err)
	}
}

// TestRevoke_AccessToken tests revoking an access token
func TestRevoke_AccessToken(t *testing.T) {
	svc, _, _, tokenStore := createTestService()

	// Create and save access token
	accessToken := &domain.OAuthAccessToken{
		ID:        "access-token-id",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Revoked:   false,
	}
	_ = tokenStore.SaveAccessToken(context.Background(), accessToken)

	// Generate JWT for the access token
	tokenString, _ := svc.generateAccessToken(
		"access-token-id",
		"user-123",
		"test-client-id",
		[]string{domain.ScopeMCPSearch},
		"http://localhost:3000",
		time.Now().Add(15*time.Minute),
	)

	req := domain.RevokeRequest{
		Token:         tokenString,
		TokenTypeHint: "access_token",
	}

	err := svc.Revoke(context.Background(), req)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Verify token was revoked
	token, _ := tokenStore.GetAccessToken(context.Background(), "access-token-id")
	if !token.Revoked {
		t.Error("expected access token to be revoked")
	}
}

// TestRevoke_RefreshToken tests revoking a refresh token
func TestRevoke_RefreshToken(t *testing.T) {
	svc, _, _, tokenStore := createTestService()

	// Create and save refresh token
	refreshToken := &domain.OAuthRefreshToken{
		ID:        "refresh-token-id",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Revoked:   false,
	}
	_ = tokenStore.SaveRefreshToken(context.Background(), refreshToken)

	req := domain.RevokeRequest{
		Token:         "refresh-token-id",
		TokenTypeHint: "refresh_token",
	}

	err := svc.Revoke(context.Background(), req)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Verify token was revoked
	token, _ := tokenStore.GetRefreshToken(context.Background(), "refresh-token-id")
	if !token.Revoked {
		t.Error("expected refresh token to be revoked")
	}
}

// TestRevoke_NotFound tests that revocation always succeeds per RFC 7009
func TestRevoke_NotFound(t *testing.T) {
	svc, _, _, _ := createTestService()

	req := domain.RevokeRequest{
		Token: "nonexistent-token",
	}

	err := svc.Revoke(context.Background(), req)
	if err != nil {
		t.Errorf("Revoke() error = %v, want nil (always succeed per RFC 7009)", err)
	}
}

// TestValidateAccessToken_Success tests validating a valid access token
func TestValidateAccessToken_Success(t *testing.T) {
	svc, _, _, tokenStore := createTestService()

	// Save access token
	accessToken := &domain.OAuthAccessToken{
		ID:        "token-id",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		Audience:  "http://localhost:3000",
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Revoked:   false,
	}
	_ = tokenStore.SaveAccessToken(context.Background(), accessToken)

	// Generate JWT
	tokenString, _ := svc.generateAccessToken(
		"token-id",
		"user-123",
		"test-client-id",
		[]string{domain.ScopeMCPSearch},
		"http://localhost:3000",
		accessToken.ExpiresAt,
	)

	info, err := svc.ValidateAccessToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if info.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", info.UserID)
	}
	if info.ClientID != "test-client-id" {
		t.Errorf("ClientID = %s, want test-client-id", info.ClientID)
	}
	if len(info.Scopes) != 1 || info.Scopes[0] != domain.ScopeMCPSearch {
		t.Errorf("Scopes = %v, want [mcp:search]", info.Scopes)
	}
	if info.Audience != "http://localhost:3000" {
		t.Errorf("Audience = %s, want http://localhost:3000", info.Audience)
	}
}

// TestValidateAccessToken_Revoked tests error for revoked token
func TestValidateAccessToken_Revoked(t *testing.T) {
	svc, _, _, tokenStore := createTestService()

	// Save revoked access token
	accessToken := &domain.OAuthAccessToken{
		ID:        "token-id",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		Audience:  "http://localhost:3000",
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Revoked:   true, // Revoked
	}
	_ = tokenStore.SaveAccessToken(context.Background(), accessToken)

	// Generate JWT
	tokenString, _ := svc.generateAccessToken(
		"token-id",
		"user-123",
		"test-client-id",
		[]string{domain.ScopeMCPSearch},
		"http://localhost:3000",
		accessToken.ExpiresAt,
	)

	_, err := svc.ValidateAccessToken(context.Background(), tokenString)
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
	if err != domain.ErrTokenRevoked {
		t.Errorf("error = %v, want ErrTokenRevoked", err)
	}
}

// TestValidateAccessToken_AudienceMismatch tests error for wrong audience
func TestValidateAccessToken_AudienceMismatch(t *testing.T) {
	svc, _, _, tokenStore := createTestService()

	// Save access token with different audience
	accessToken := &domain.OAuthAccessToken{
		ID:        "token-id",
		ClientID:  "test-client-id",
		UserID:    "user-123",
		Scopes:    []string{domain.ScopeMCPSearch},
		Audience:  "http://different-server.com",
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Revoked:   false,
	}
	_ = tokenStore.SaveAccessToken(context.Background(), accessToken)

	// Generate JWT with wrong audience
	tokenString, _ := svc.generateAccessToken(
		"token-id",
		"user-123",
		"test-client-id",
		[]string{domain.ScopeMCPSearch},
		"http://different-server.com", // Wrong audience
		accessToken.ExpiresAt,
	)

	_, err := svc.ValidateAccessToken(context.Background(), tokenString)
	if err == nil {
		t.Fatal("expected error for audience mismatch")
	}
	if !strings.Contains(err.Error(), "audience") {
		t.Errorf("error = %v, want error mentioning audience", err)
	}
}

// TestGetServerMetadata tests OAuth server metadata
func TestGetServerMetadata(t *testing.T) {
	svc, _, _, _ := createTestService()

	baseURL := "https://sercha.example.com"
	metadata := svc.GetServerMetadata(baseURL)

	if metadata.Issuer != baseURL {
		t.Errorf("Issuer = %s, want %s", metadata.Issuer, baseURL)
	}
	if metadata.AuthorizationEndpoint != baseURL+"/oauth/authorize" {
		t.Errorf("AuthorizationEndpoint = %s, want %s/oauth/authorize", metadata.AuthorizationEndpoint, baseURL)
	}
	if metadata.TokenEndpoint != baseURL+"/oauth/token" {
		t.Errorf("TokenEndpoint = %s, want %s/oauth/token", metadata.TokenEndpoint, baseURL)
	}
	if metadata.RegistrationEndpoint != baseURL+"/oauth/register" {
		t.Errorf("RegistrationEndpoint = %s, want %s/oauth/register", metadata.RegistrationEndpoint, baseURL)
	}
	if metadata.RevocationEndpoint != baseURL+"/oauth/revoke" {
		t.Errorf("RevocationEndpoint = %s, want %s/oauth/revoke", metadata.RevocationEndpoint, baseURL)
	}

	if len(metadata.ResponseTypesSupported) != 1 || metadata.ResponseTypesSupported[0] != "code" {
		t.Errorf("ResponseTypesSupported = %v, want [code]", metadata.ResponseTypesSupported)
	}

	if len(metadata.GrantTypesSupported) != 2 {
		t.Errorf("GrantTypesSupported length = %d, want 2", len(metadata.GrantTypesSupported))
	}

	if len(metadata.CodeChallengeMethodsSupported) != 1 || metadata.CodeChallengeMethodsSupported[0] != "S256" {
		t.Errorf("CodeChallengeMethodsSupported = %v, want [S256]", metadata.CodeChallengeMethodsSupported)
	}

	if len(metadata.TokenEndpointAuthMethodsSupported) != 2 {
		t.Errorf("TokenEndpointAuthMethodsSupported length = %d, want 2", len(metadata.TokenEndpointAuthMethodsSupported))
	}

	if len(metadata.ScopesSupported) != 3 {
		t.Errorf("ScopesSupported length = %d, want 3", len(metadata.ScopesSupported))
	}

	if !metadata.ResourceIndicatorsSupported {
		t.Error("expected ResourceIndicatorsSupported to be true")
	}
}

// TestGenerateAccessToken tests JWT generation
func TestGenerateAccessToken(t *testing.T) {
	svc, _, _, _ := createTestService()

	expiresAt := time.Now().Add(15 * time.Minute)
	tokenString, err := svc.generateAccessToken(
		"token-id",
		"user-123",
		"client-123",
		[]string{"scope1", "scope2"},
		"http://localhost:3000",
		expiresAt,
	)
	if err != nil {
		t.Fatalf("generateAccessToken() error = %v", err)
	}

	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-jwt-secret-key-for-testing"), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to parse claims")
	}

	if claims["jti"] != "token-id" {
		t.Errorf("jti = %v, want token-id", claims["jti"])
	}
	if claims["sub"] != "user-123" {
		t.Errorf("sub = %v, want user-123", claims["sub"])
	}
	if claims["client_id"] != "client-123" {
		t.Errorf("client_id = %v, want client-123", claims["client_id"])
	}
	if claims["scope"] != "scope1 scope2" {
		t.Errorf("scope = %v, want 'scope1 scope2'", claims["scope"])
	}
	if claims["aud"] != "http://localhost:3000" {
		t.Errorf("aud = %v, want http://localhost:3000", claims["aud"])
	}
}

// TestValidatePKCE tests PKCE validation
func TestValidatePKCE(t *testing.T) {
	tests := []struct {
		name      string
		verifier  string
		challenge string
		expected  bool
	}{
		{
			name:     "valid PKCE",
			verifier: "test-verifier-1234567890",
			challenge: func() string {
				hash := sha256.Sum256([]byte("test-verifier-1234567890"))
				return base64.RawURLEncoding.EncodeToString(hash[:])
			}(),
			expected: true,
		},
		{
			name:     "invalid verifier",
			verifier: "wrong-verifier",
			challenge: func() string {
				hash := sha256.Sum256([]byte("test-verifier-1234567890"))
				return base64.RawURLEncoding.EncodeToString(hash[:])
			}(),
			expected: false,
		},
		{
			name:      "empty verifier",
			verifier:  "",
			challenge: "some-challenge",
			expected:  false,
		},
		{
			name:      "empty challenge",
			verifier:  "some-verifier",
			challenge: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePKCE(tt.verifier, tt.challenge)
			if result != tt.expected {
				t.Errorf("validatePKCE() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Tests for GetClientPublicInfo

func TestGetClientPublicInfo_Success(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	client := &domain.OAuthClient{
		ID:              "test-client-id",
		Name:            "My MCP Client",
		ApplicationType: "native",
		Active:          true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	info, err := svc.GetClientPublicInfo(context.Background(), "test-client-id")
	if err != nil {
		t.Fatalf("GetClientPublicInfo() error = %v", err)
	}
	if info.ClientID != "test-client-id" {
		t.Errorf("ClientID = %s, want test-client-id", info.ClientID)
	}
	if info.Name != "My MCP Client" {
		t.Errorf("Name = %s, want My MCP Client", info.Name)
	}
	if info.ApplicationType != "native" {
		t.Errorf("ApplicationType = %s, want native", info.ApplicationType)
	}
}

func TestGetClientPublicInfo_NotFound(t *testing.T) {
	svc, _, _, _ := createTestService()

	_, err := svc.GetClientPublicInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent client")
	}
	if err != domain.ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestGetClientPublicInfo_InactiveClient(t *testing.T) {
	svc, clientStore, _, _ := createTestService()

	client := &domain.OAuthClient{
		ID:              "inactive-client",
		Name:            "Deactivated App",
		ApplicationType: "web",
		Active:          false,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_ = clientStore.Save(context.Background(), client)

	_, err := svc.GetClientPublicInfo(context.Background(), "inactive-client")
	if err == nil {
		t.Fatal("expected error for inactive client")
	}
	if err != domain.ErrNotFound {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
