package notion

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure OAuthHandler implements the interface.
var _ connectors.OAuthHandler = (*OAuthHandler)(nil)

// OAuthHandler handles OAuth operations for Notion.
type OAuthHandler struct {
	httpClient *http.Client
}

// NewOAuthHandler creates a new Notion OAuth handler.
func NewOAuthHandler() *OAuthHandler {
	return &OAuthHandler{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// BuildAuthURL constructs the Notion OAuth authorization URL.
// Notion OAuth uses owner=user parameter to specify workspace selection.
func (h *OAuthHandler) BuildAuthURL(clientID, redirectURI, state, codeChallenge string, scopes []string) string {
	params := url.Values{
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
		"response_type": {"code"},
		"owner":        {"user"}, // Notion-specific: allows user to select workspace
		"state":        {state},
	}

	// Notion doesn't use traditional scopes - permissions are granted at the page/database level
	// during the OAuth consent flow

	return "https://api.notion.com/v1/oauth/authorize?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
// IMPORTANT: Notion uses HTTP Basic authentication, NOT form POST.
func (h *OAuthHandler) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
	body := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.notion.com/v1/oauth/token",
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// HTTP Basic Auth: base64(CLIENT_ID:CLIENT_SECRET)
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
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("oauth error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Notion tokens don't expire, so we set ExpiresIn to 0
	return &driven.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: "", // Notion doesn't provide refresh tokens
		TokenType:    tokenResp.TokenType,
		Scope:        "", // Notion doesn't use traditional scopes
		ExpiresIn:    0,  // Tokens don't expire
	}, nil
}

// RefreshToken refreshes an expired access token.
// NOTE: Notion tokens don't expire, so this always returns nil.
func (h *OAuthHandler) RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*driven.OAuthToken, error) {
	// Notion tokens don't expire and don't have refresh tokens
	return nil, fmt.Errorf("notion tokens do not expire and cannot be refreshed")
}

// GetUserInfo fetches the authenticated user's information.
// For Notion, we call /v1/users/me to get workspace and bot information.
func (h *OAuthHandler) GetUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.notion.com/v1/users/me", nil)
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

	// Extract email from person if available
	if user.Person != nil {
		userInfo.Email = user.Person.Email
	}

	// If this is a bot, use workspace name as the display name
	if user.Type == "bot" && user.Bot != nil {
		if user.Bot.WorkspaceName != "" {
			userInfo.Name = user.Bot.WorkspaceName
		}
	}

	return userInfo, nil
}

// DefaultConfig returns Notion's default OAuth configuration.
func (h *OAuthHandler) DefaultConfig() connectors.OAuthDefaults {
	return connectors.OAuthDefaults{
		AuthURL:      "https://api.notion.com/v1/oauth/authorize",
		TokenURL:     "https://api.notion.com/v1/oauth/token",
		Scopes:       []string{}, // Notion doesn't use traditional scopes
		UserInfoURL:  "https://api.notion.com/v1/users/me",
		SupportsPKCE: false, // Notion doesn't support PKCE
	}
}
