package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Mock OAuth Server Service for testing
type mockOAuthServerService struct {
	registerClientFn      func(ctx context.Context, req domain.ClientRegistrationRequest) (*domain.ClientRegistrationResponse, error)
	authorizeFn           func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error)
	tokenFn               func(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error)
	revokeFn              func(ctx context.Context, req domain.RevokeRequest) error
	validateAccessTokenFn func(ctx context.Context, token string) (*driving.OAuthTokenInfo, error)
	getServerMetadataFn   func(baseURL string) *driving.OAuthServerMetadata
}

func (m *mockOAuthServerService) RegisterClient(ctx context.Context, req domain.ClientRegistrationRequest) (*domain.ClientRegistrationResponse, error) {
	if m.registerClientFn != nil {
		return m.registerClientFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOAuthServerService) Authorize(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
	if m.authorizeFn != nil {
		return m.authorizeFn(ctx, userID, req)
	}
	return "", errors.New("not implemented")
}

func (m *mockOAuthServerService) Token(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error) {
	if m.tokenFn != nil {
		return m.tokenFn(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOAuthServerService) Revoke(ctx context.Context, req domain.RevokeRequest) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, req)
	}
	return nil
}

func (m *mockOAuthServerService) ValidateAccessToken(ctx context.Context, token string) (*driving.OAuthTokenInfo, error) {
	if m.validateAccessTokenFn != nil {
		return m.validateAccessTokenFn(ctx, token)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOAuthServerService) GetServerMetadata(baseURL string) *driving.OAuthServerMetadata {
	if m.getServerMetadataFn != nil {
		return m.getServerMetadataFn(baseURL)
	}
	return &driving.OAuthServerMetadata{
		Issuer:                baseURL,
		AuthorizationEndpoint: baseURL + "/oauth/authorize",
		TokenEndpoint:         baseURL + "/oauth/token",
	}
}

// Tests for GET /oauth/authorize

func TestHandleOAuthServerAuthorize_ServiceNotConfigured(t *testing.T) {
	server := &Server{
		oauthServerService: nil,
	}

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=test&redirect_uri=http://localhost", nil)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorize(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "oauth server not configured" {
		t.Errorf("expected error 'oauth server not configured', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorize_MissingClientID(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
		uiBaseURL:          "http://localhost:3000",
	}

	req := httptest.NewRequest("GET", "/oauth/authorize?redirect_uri=http://localhost", nil)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorize(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "client_id is required" {
		t.Errorf("expected error 'client_id is required', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorize_MissingRedirectURI(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
		uiBaseURL:          "http://localhost:3000",
	}

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=test-client", nil)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorize(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "redirect_uri is required" {
		t.Errorf("expected error 'redirect_uri is required', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorize_RedirectsToFrontend(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
		uiBaseURL:          "http://localhost:3000",
	}

	// Build OAuth authorize request with all parameters
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", "test-client-id")
	params.Set("redirect_uri", "http://localhost:8080/callback")
	params.Set("scope", "mcp:search mcp:documents:read")
	params.Set("state", "random-state-123")
	params.Set("code_challenge", "test-challenge")
	params.Set("code_challenge_method", "S256")
	params.Set("resource", "http://localhost:8080/mcp")

	req := httptest.NewRequest("GET", "/oauth/authorize?"+params.Encode(), nil)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorize(rr, req)

	// Should redirect (302)
	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	// Check Location header
	location := rr.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header, got none")
	}

	// Parse redirect URL
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	// Verify base URL is the frontend
	expectedBase := "http://localhost:3000/oauth/authorize"
	actualBase := redirectURL.Scheme + "://" + redirectURL.Host + redirectURL.Path
	if actualBase != expectedBase {
		t.Errorf("expected redirect to %s, got %s", expectedBase, actualBase)
	}

	// Verify all params are preserved
	query := redirectURL.Query()
	if query.Get("response_type") != "code" {
		t.Errorf("expected response_type 'code', got %s", query.Get("response_type"))
	}
	if query.Get("client_id") != "test-client-id" {
		t.Errorf("expected client_id 'test-client-id', got %s", query.Get("client_id"))
	}
	if query.Get("redirect_uri") != "http://localhost:8080/callback" {
		t.Errorf("expected redirect_uri 'http://localhost:8080/callback', got %s", query.Get("redirect_uri"))
	}
	if query.Get("scope") != "mcp:search mcp:documents:read" {
		t.Errorf("expected scope 'mcp:search mcp:documents:read', got %s", query.Get("scope"))
	}
	if query.Get("state") != "random-state-123" {
		t.Errorf("expected state 'random-state-123', got %s", query.Get("state"))
	}
	if query.Get("code_challenge") != "test-challenge" {
		t.Errorf("expected code_challenge 'test-challenge', got %s", query.Get("code_challenge"))
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Errorf("expected code_challenge_method 'S256', got %s", query.Get("code_challenge_method"))
	}
	if query.Get("resource") != "http://localhost:8080/mcp" {
		t.Errorf("expected resource 'http://localhost:8080/mcp', got %s", query.Get("resource"))
	}
}

func TestHandleOAuthServerAuthorize_RedirectsWithMinimalParams(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
		uiBaseURL:          "http://localhost:3000",
	}

	// Only required params
	params := url.Values{}
	params.Set("client_id", "test-client")
	params.Set("redirect_uri", "http://localhost/callback")

	req := httptest.NewRequest("GET", "/oauth/authorize?"+params.Encode(), nil)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorize(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	redirectURL, _ := url.Parse(location)
	query := redirectURL.Query()

	// Verify required params are present
	if query.Get("client_id") != "test-client" {
		t.Errorf("expected client_id 'test-client', got %s", query.Get("client_id"))
	}
	if query.Get("redirect_uri") != "http://localhost/callback" {
		t.Errorf("expected redirect_uri 'http://localhost/callback', got %s", query.Get("redirect_uri"))
	}
}

// Tests for POST /oauth/authorize/complete

func TestHandleOAuthServerAuthorizeComplete_ServiceNotConfigured(t *testing.T) {
	server := &Server{
		oauthServerService: nil,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleOAuthServerAuthorizeComplete_InvalidJSON(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", response["error"])
	}
	if response["error_description"] != "invalid request body" {
		t.Errorf("expected error_description 'invalid request body', got %s", response["error_description"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_MissingRedirectURI(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID: "test-client",
		// RedirectURI is missing
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_NoAuthContext(t *testing.T) {
	mockOAuth := &mockOAuthServerService{}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
	})
	// No auth context in request (middleware would normally add this)
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "missing authorization token" {
		t.Errorf("expected error 'missing authorization token', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_ServiceError_UnsupportedResponseType(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "", errors.New("unsupported response_type")
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ResponseType: "token",
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	// Add auth context
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "unsupported_response_type" {
		t.Errorf("expected error 'unsupported_response_type', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_ServiceError_UnauthorizedClient(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "", errors.New("client not found")
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "invalid-client",
		RedirectURI: "http://localhost/callback",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "unauthorized_client" {
		t.Errorf("expected error 'unauthorized_client', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_ServiceError_InvalidScope(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "", errors.New("invalid scope requested")
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
		Scope:       "invalid:scope",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid_scope" {
		t.Errorf("expected error 'invalid_scope', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_ServiceError_InvalidRedirectURI(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "", errors.New("redirect_uri not registered")
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "http://evil.com/callback",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_ServiceError_InvalidCodeChallenge(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "", errors.New("code_challenge is required")
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
		// Missing code_challenge
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", response["error"])
	}
}

func TestHandleOAuthServerAuthorizeComplete_Success(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			if userID != "user-123" {
				t.Errorf("expected userID 'user-123', got %s", userID)
			}
			if req.ClientID != "test-client" {
				t.Errorf("expected ClientID 'test-client', got %s", req.ClientID)
			}
			return "auth-code-xyz", nil
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ResponseType:        "code",
		ClientID:            "test-client",
		RedirectURI:         "http://localhost/callback",
		Scope:               "mcp:search",
		State:               "state-123",
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleAdmin,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	redirectURL := response["redirect_url"]
	if redirectURL == "" {
		t.Fatal("expected redirect_url, got empty string")
	}

	// Parse redirect URL
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("failed to parse redirect_url: %v", err)
	}

	// Verify base URL
	expectedBase := "http://localhost/callback"
	actualBase := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path
	if actualBase != expectedBase {
		t.Errorf("expected redirect base %s, got %s", expectedBase, actualBase)
	}

	// Verify code and state in query params
	query := parsedURL.Query()
	if query.Get("code") != "auth-code-xyz" {
		t.Errorf("expected code 'auth-code-xyz', got %s", query.Get("code"))
	}
	if query.Get("state") != "state-123" {
		t.Errorf("expected state 'state-123', got %s", query.Get("state"))
	}
}

func TestHandleOAuthServerAuthorizeComplete_SuccessWithoutState(t *testing.T) {
	mockOAuth := &mockOAuthServerService{
		authorizeFn: func(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
			return "auth-code-abc", nil
		},
	}
	server := &Server{
		oauthServerService: mockOAuth,
	}

	body, _ := json.Marshal(domain.AuthorizeRequest{
		ClientID:      "test-client",
		RedirectURI:   "http://localhost/callback",
		CodeChallenge: "challenge",
	})
	req := httptest.NewRequest("POST", "/oauth/authorize/complete", bytes.NewBuffer(body))
	authCtx := &domain.AuthContext{
		UserID: "user-123",
		Role:   domain.RoleViewer,
	}
	ctx := context.WithValue(req.Context(), authContextKey, authCtx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleOAuthServerAuthorizeComplete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	parsedURL, _ := url.Parse(response["redirect_url"])
	query := parsedURL.Query()

	if query.Get("code") != "auth-code-abc" {
		t.Errorf("expected code 'auth-code-abc', got %s", query.Get("code"))
	}
	// State should not be in query params if not provided
	if query.Get("state") != "" {
		t.Errorf("expected no state param, got %s", query.Get("state"))
	}
}

// Tests for writeOAuthError helper

func TestWriteOAuthError_WithDescription(t *testing.T) {
	rr := httptest.NewRecorder()

	writeOAuthError(rr, "invalid_request", "Missing client_id parameter")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	// Check headers
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got %s", rr.Header().Get("Cache-Control"))
	}
	if rr.Header().Get("Pragma") != "no-cache" {
		t.Errorf("expected Pragma 'no-cache', got %s", rr.Header().Get("Pragma"))
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", response["error"])
	}
	if response["error_description"] != "Missing client_id parameter" {
		t.Errorf("expected error_description 'Missing client_id parameter', got %s", response["error_description"])
	}
}

func TestWriteOAuthError_WithoutDescription(t *testing.T) {
	rr := httptest.NewRecorder()

	writeOAuthError(rr, "server_error", "")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "server_error" {
		t.Errorf("expected error 'server_error', got %s", response["error"])
	}
	// error_description should not be present when empty
	if _, exists := response["error_description"]; exists {
		t.Errorf("expected no error_description, got %s", response["error_description"])
	}
}

func TestWriteOAuthError_SetsCorrectHeaders(t *testing.T) {
	rr := httptest.NewRecorder()

	writeOAuthError(rr, "access_denied", "User denied consent")

	// Verify headers required by OAuth 2.0 spec
	cacheControl := rr.Header().Get("Cache-Control")
	pragma := rr.Header().Get("Pragma")

	if cacheControl != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got %s", cacheControl)
	}
	if pragma != "no-cache" {
		t.Errorf("expected Pragma 'no-cache', got %s", pragma)
	}
}
