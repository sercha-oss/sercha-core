package microsoft

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

func TestOAuthHandler_BuildAuthURL(t *testing.T) {
	handler := NewOAuthHandler()

	clientID := "test-client-id"
	redirectURI := "https://example.com/callback"
	state := "random-state"
	codeChallenge := "challenge-123"
	scopes := []string{"Files.Read", "User.Read", "offline_access"}

	authURL := handler.BuildAuthURL(clientID, redirectURI, state, codeChallenge, scopes)

	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("BuildAuthURL() returned invalid URL: %v", err)
	}

	if parsedURL.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", parsedURL.Scheme)
	}

	if parsedURL.Host != "login.microsoftonline.com" {
		t.Errorf("Host = %q, want login.microsoftonline.com", parsedURL.Host)
	}

	if parsedURL.Path != "/common/oauth2/v2.0/authorize" {
		t.Errorf("Path = %q, want /common/oauth2/v2.0/authorize", parsedURL.Path)
	}

	query := parsedURL.Query()

	if query.Get("client_id") != clientID {
		t.Errorf("client_id = %q, want %q", query.Get("client_id"), clientID)
	}

	if query.Get("redirect_uri") != redirectURI {
		t.Errorf("redirect_uri = %q, want %q", query.Get("redirect_uri"), redirectURI)
	}

	if query.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", query.Get("response_type"))
	}

	if query.Get("state") != state {
		t.Errorf("state = %q, want %q", query.Get("state"), state)
	}

	if query.Get("response_mode") != "query" {
		t.Errorf("response_mode = %q, want query", query.Get("response_mode"))
	}

	if query.Get("code_challenge") != codeChallenge {
		t.Errorf("code_challenge = %q, want %q", query.Get("code_challenge"), codeChallenge)
	}

	if query.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", query.Get("code_challenge_method"))
	}

	expectedScope := "Files.Read User.Read offline_access"
	if query.Get("scope") != expectedScope {
		t.Errorf("scope = %q, want %q", query.Get("scope"), expectedScope)
	}
}

func TestOAuthHandler_ExchangeCode(t *testing.T) {
	expectedClientID := "client-123"
	expectedClientSecret := "secret-456"
	expectedCode := "auth-code-789"
	expectedRedirectURI := "https://example.com/callback"
	expectedCodeVerifier := "verifier-abc"

	paramsChecked := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		if r.URL.Path != "/common/oauth2/v2.0/token" {
			t.Errorf("Path = %q, want /common/oauth2/v2.0/token", r.URL.Path)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", contentType)
		}

		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))

		if params.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", params.Get("grant_type"))
		}

		if params.Get("code") != expectedCode {
			t.Errorf("code = %q, want %q", params.Get("code"), expectedCode)
		}

		if params.Get("redirect_uri") != expectedRedirectURI {
			t.Errorf("redirect_uri = %q, want %q", params.Get("redirect_uri"), expectedRedirectURI)
		}

		if params.Get("client_id") != expectedClientID {
			t.Errorf("client_id = %q, want %q", params.Get("client_id"), expectedClientID)
		}

		if params.Get("client_secret") != expectedClientSecret {
			t.Errorf("client_secret = %q, want %q", params.Get("client_secret"), expectedClientSecret)
		}

		if params.Get("code_verifier") != expectedCodeVerifier {
			t.Errorf("code_verifier = %q, want %q", params.Get("code_verifier"), expectedCodeVerifier)
		} else {
			paramsChecked = true
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-token-abc",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         "Files.Read User.Read offline_access",
			"refresh_token": "refresh-token-xyz",
		})
	}))
	defer ts.Close()

	// Create a testable OAuth handler
	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/common/oauth2/v2.0/token",
	}

	token, err := handler.exchangeCode(context.Background(), expectedClientID, expectedClientSecret, expectedCode, expectedRedirectURI, expectedCodeVerifier)
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}

	if !paramsChecked {
		t.Error("Request parameters were not checked")
	}

	if token.AccessToken != "access-token-abc" {
		t.Errorf("AccessToken = %q, want access-token-abc", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", token.TokenType)
	}

	if token.RefreshToken != "refresh-token-xyz" {
		t.Errorf("RefreshToken = %q, want refresh-token-xyz", token.RefreshToken)
	}

	if token.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", token.ExpiresIn)
	}

	if token.Scope != "Files.Read User.Read offline_access" {
		t.Errorf("Scope = %q, want Files.Read User.Read offline_access", token.Scope)
	}
}

func TestOAuthHandler_ExchangeCode_WithoutPKCE(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))

		if params.Get("code_verifier") != "" {
			t.Errorf("code_verifier should be empty when not provided")
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/common/oauth2/v2.0/token",
	}

	// Call without code verifier (empty string)
	_, err := handler.exchangeCode(context.Background(), "client", "secret", "code", "http://localhost/callback", "")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
}

func TestOAuthHandler_ExchangeCode_Error(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   map[string]string
		wantErrContain string
	}{
		{
			name:       "invalid_grant",
			statusCode: http.StatusBadRequest,
			responseBody: map[string]string{
				"error":             "invalid_grant",
				"error_description": "Code is invalid or expired",
			},
			wantErrContain: "invalid_grant",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			responseBody: map[string]string{
				"error":             "invalid_client",
				"error_description": "Client authentication failed",
			},
			wantErrContain: "invalid_client",
		},
		{
			name:           "server_error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   map[string]string{},
			wantErrContain: "token exchange failed (500)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer ts.Close()

			handler := &testOAuthHandler{
				OAuthHandler: &OAuthHandler{
					httpClient: ts.Client(),
				},
				tokenURL: ts.URL + "/common/oauth2/v2.0/token",
			}

			_, err := handler.exchangeCode(context.Background(), "client", "secret", "code", "http://localhost/callback", "")
			if err == nil {
				t.Fatal("ExchangeCode() expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrContain)
			}
		})
	}
}

func TestOAuthHandler_RefreshToken(t *testing.T) {
	expectedClientID := "client-123"
	expectedClientSecret := "secret-456"
	expectedRefreshToken := "refresh-token-xyz"

	paramsChecked := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		if r.URL.Path != "/common/oauth2/v2.0/token" {
			t.Errorf("Path = %q, want /common/oauth2/v2.0/token", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))

		if params.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", params.Get("grant_type"))
		}

		if params.Get("refresh_token") != expectedRefreshToken {
			t.Errorf("refresh_token = %q, want %q", params.Get("refresh_token"), expectedRefreshToken)
		}

		if params.Get("client_id") != expectedClientID {
			t.Errorf("client_id = %q, want %q", params.Get("client_id"), expectedClientID)
		}

		if params.Get("client_secret") != expectedClientSecret {
			t.Errorf("client_secret = %q, want %q", params.Get("client_secret"), expectedClientSecret)
		} else {
			paramsChecked = true
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         "Files.Read User.Read offline_access",
			"refresh_token": "new-refresh-token",
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/common/oauth2/v2.0/token",
	}

	token, err := handler.refreshToken(context.Background(), expectedClientID, expectedClientSecret, expectedRefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if !paramsChecked {
		t.Error("Request parameters were not checked")
	}

	if token.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want new-access-token", token.AccessToken)
	}

	if token.RefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken = %q, want new-refresh-token", token.RefreshToken)
	}

	if token.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", token.ExpiresIn)
	}
}

func TestOAuthHandler_RefreshToken_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Refresh token is invalid or expired",
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/common/oauth2/v2.0/token",
	}

	_, err := handler.refreshToken(context.Background(), "client", "secret", "invalid-refresh-token")
	if err == nil {
		t.Fatal("RefreshToken() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error = %q, want to contain 'invalid_grant'", err.Error())
	}
}

func TestOAuthHandler_GetUserInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me" {
			t.Errorf("Path = %q, want /v1.0/me", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:                "user-123",
			DisplayName:       "John Doe",
			UserPrincipalName: "john@example.com",
			Mail:              "john.doe@example.com",
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		userInfoURL: ts.URL + "/v1.0/me",
	}

	userInfo, err := handler.getUserInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}

	if userInfo.ID != "user-123" {
		t.Errorf("ID = %q, want user-123", userInfo.ID)
	}

	if userInfo.Name != "John Doe" {
		t.Errorf("Name = %q, want John Doe", userInfo.Name)
	}

	if userInfo.Email != "john@example.com" {
		t.Errorf("Email = %q, want john@example.com", userInfo.Email)
	}

	if userInfo.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty (Microsoft Graph /me doesn't return avatar)", userInfo.ImageURL)
	}
}

func TestOAuthHandler_GetUserInfo_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid token"))
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		userInfoURL: ts.URL + "/v1.0/me",
	}

	_, err := handler.getUserInfo(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("GetUserInfo() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want to contain '401'", err.Error())
	}
}

func TestOAuthHandler_DefaultConfig(t *testing.T) {
	handler := NewOAuthHandler()
	config := handler.DefaultConfig()

	if config.AuthURL != "https://login.microsoftonline.com/common/oauth2/v2.0/authorize" {
		t.Errorf("AuthURL = %q, want https://login.microsoftonline.com/common/oauth2/v2.0/authorize", config.AuthURL)
	}

	if config.TokenURL != "https://login.microsoftonline.com/common/oauth2/v2.0/token" {
		t.Errorf("TokenURL = %q, want https://login.microsoftonline.com/common/oauth2/v2.0/token", config.TokenURL)
	}

	if config.UserInfoURL != "https://graph.microsoft.com/v1.0/me" {
		t.Errorf("UserInfoURL = %q, want https://graph.microsoft.com/v1.0/me", config.UserInfoURL)
	}

	expectedScopes := []string{"Files.Read", "User.Read", "offline_access"}
	if len(config.Scopes) != len(expectedScopes) {
		t.Errorf("Scopes length = %d, want %d", len(config.Scopes), len(expectedScopes))
	}
	for i, scope := range expectedScopes {
		if config.Scopes[i] != scope {
			t.Errorf("Scopes[%d] = %q, want %q", i, config.Scopes[i], scope)
		}
	}

	if !config.SupportsPKCE {
		t.Error("SupportsPKCE = false, want true (Microsoft supports PKCE)")
	}
}

// testOAuthHandler wraps OAuthHandler for testing with custom URLs
type testOAuthHandler struct {
	*OAuthHandler
	tokenURL    string
	userInfoURL string
}

func (h *testOAuthHandler) exchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
	params := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	if codeVerifier != "" {
		params.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("oauth error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &driven.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

func (h *testOAuthHandler) refreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*driven.OAuthToken, error) {
	params := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("oauth error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &driven.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

func (h *testOAuthHandler) getUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", h.userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user info failed (%d): %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	return &driven.OAuthUserInfo{
		ID:       user.ID,
		Email:    user.UserPrincipalName,
		Name:     user.DisplayName,
		ImageURL: "",
	}, nil
}
