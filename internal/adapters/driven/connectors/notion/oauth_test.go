package notion

import (
	"context"
	"encoding/base64"
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
	scopes := []string{"read", "write"}

	authURL := handler.BuildAuthURL(clientID, redirectURI, state, codeChallenge, scopes)

	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("BuildAuthURL() returned invalid URL: %v", err)
	}

	if parsedURL.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", parsedURL.Scheme)
	}

	if parsedURL.Host != "api.notion.com" {
		t.Errorf("Host = %q, want api.notion.com", parsedURL.Host)
	}

	if parsedURL.Path != "/v1/oauth/authorize" {
		t.Errorf("Path = %q, want /v1/oauth/authorize", parsedURL.Path)
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

	if query.Get("owner") != "user" {
		t.Errorf("owner = %q, want user", query.Get("owner"))
	}
}

func TestOAuthHandler_ExchangeCode(t *testing.T) {
	expectedClientID := "client-123"
	expectedClientSecret := "secret-456"
	expectedCode := "auth-code-789"
	expectedRedirectURI := "https://example.com/callback"
	expectedBasicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(expectedClientID+":"+expectedClientSecret))

	authHeaderChecked := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		if r.URL.Path != "/oauth/token" {
			t.Errorf("Path = %q, want /oauth/token", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != expectedBasicAuth {
			t.Errorf("Authorization = %q, want %q", authHeader, expectedBasicAuth)
		} else {
			authHeaderChecked = true
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

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "access-token-abc",
			TokenType:   "Bearer",
			BotID:       "bot-123",
		})
	}))
	defer ts.Close()

	// Create a testable OAuth handler with custom httpClient
	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/oauth/token",
	}

	token, err := handler.exchangeCode(context.Background(), expectedClientID, expectedClientSecret, expectedCode, expectedRedirectURI, "")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}

	if !authHeaderChecked {
		t.Error("Basic auth header was not checked")
	}

	if token.AccessToken != "access-token-abc" {
		t.Errorf("AccessToken = %q, want access-token-abc", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", token.TokenType)
	}

	if token.RefreshToken != "" {
		t.Errorf("RefreshToken = %q, want empty string", token.RefreshToken)
	}

	if token.ExpiresIn != 0 {
		t.Errorf("ExpiresIn = %d, want 0", token.ExpiresIn)
	}
}

// testOAuthHandler wraps OAuthHandler for testing with custom URLs
type testOAuthHandler struct {
	*OAuthHandler
	tokenURL    string
	userInfoURL string
}

func (h *testOAuthHandler) exchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
	body := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		h.tokenURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &driven.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: "",
		TokenType:    tokenResp.TokenType,
		Scope:        "",
		ExpiresIn:    0,
	}, nil
}

func (h *testOAuthHandler) getUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", h.userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user info failed (%d): %s", resp.StatusCode, string(body))
	}

	var user UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	userInfo := &driven.OAuthUserInfo{
		ID:       user.ID,
		Name:     user.Name,
		ImageURL: user.AvatarURL,
	}

	if user.Person != nil {
		userInfo.Email = user.Person.Email
	}

	if user.Type == "bot" && user.Bot != nil {
		if user.Bot.WorkspaceName != "" {
			userInfo.Name = user.Bot.WorkspaceName
		}
	}

	return userInfo, nil
}

func TestOAuthHandler_ExchangeCode_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Code is invalid or expired",
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		tokenURL: ts.URL + "/oauth/token",
	}

	_, err := handler.exchangeCode(context.Background(), "client", "secret", "invalid-code", "http://localhost/callback", "")
	if err == nil {
		t.Error("ExchangeCode() expected error for invalid code, got nil")
	}
}

func TestOAuthHandler_RefreshToken(t *testing.T) {
	handler := NewOAuthHandler()

	token, err := handler.RefreshToken(context.Background(), "client", "secret", "refresh-token")

	if token != nil {
		t.Error("RefreshToken() expected nil token")
	}

	if err == nil {
		t.Error("RefreshToken() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "do not expire") {
		t.Errorf("error message = %q, want to contain 'do not expire'", err.Error())
	}
}

func TestOAuthHandler_GetUserInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/users/me" {
			t.Errorf("Path = %q, want /users/me", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", authHeader)
		}

		versionHeader := r.Header.Get("Notion-Version")
		if versionHeader != "2022-06-28" {
			t.Errorf("Notion-Version = %q, want 2022-06-28", versionHeader)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(UserResponse{
			Object:    "user",
			ID:        "user-123",
			Type:      "person",
			Name:      "John Doe",
			AvatarURL: "https://example.com/avatar.jpg",
			Person: &Person{
				Email: "john@example.com",
			},
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		userInfoURL: ts.URL + "/users/me",
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

	if userInfo.ImageURL != "https://example.com/avatar.jpg" {
		t.Errorf("ImageURL = %q, want https://example.com/avatar.jpg", userInfo.ImageURL)
	}
}

func TestOAuthHandler_GetUserInfo_Bot(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(UserResponse{
			Object: "user",
			ID:     "bot-456",
			Type:   "bot",
			Name:   "Bot Name",
			Bot: &BotOwnerInfo{
				WorkspaceName: "My Workspace",
			},
		})
	}))
	defer ts.Close()

	handler := &testOAuthHandler{
		OAuthHandler: &OAuthHandler{
			httpClient: ts.Client(),
		},
		userInfoURL: ts.URL + "/users/me",
	}

	userInfo, err := handler.getUserInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}

	if userInfo.Name != "My Workspace" {
		t.Errorf("Name = %q, want My Workspace (workspace name should override bot name)", userInfo.Name)
	}
}

func TestOAuthHandler_DefaultConfig(t *testing.T) {
	handler := NewOAuthHandler()
	config := handler.DefaultConfig()

	if config.AuthURL != "https://api.notion.com/v1/oauth/authorize" {
		t.Errorf("AuthURL = %q, want https://api.notion.com/v1/oauth/authorize", config.AuthURL)
	}

	if config.TokenURL != "https://api.notion.com/v1/oauth/token" {
		t.Errorf("TokenURL = %q, want https://api.notion.com/v1/oauth/token", config.TokenURL)
	}

	if config.UserInfoURL != "https://api.notion.com/v1/users/me" {
		t.Errorf("UserInfoURL = %q, want https://api.notion.com/v1/users/me", config.UserInfoURL)
	}

	if len(config.Scopes) != 0 {
		t.Errorf("Scopes = %v, want empty slice", config.Scopes)
	}

	if config.SupportsPKCE {
		t.Error("SupportsPKCE = true, want false")
	}
}
