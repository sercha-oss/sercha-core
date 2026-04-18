package microsoft

import (
	"context"
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

// OAuthHandler handles OAuth operations for Microsoft Azure AD / Microsoft Graph.
// This handler is shared across all Microsoft services (OneDrive, SharePoint, Teams, etc.).
type OAuthHandler struct {
	httpClient *http.Client
}

// NewOAuthHandler creates a new Microsoft OAuth handler.
func NewOAuthHandler() *OAuthHandler {
	return &OAuthHandler{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// BuildAuthURL constructs the Microsoft Azure AD OAuth authorization URL.
// Microsoft OAuth uses the v2.0 endpoint with PKCE support.
func (h *OAuthHandler) BuildAuthURL(clientID, redirectURI, state, codeChallenge string, scopes []string) string {
	params := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"scope":                 {strings.Join(scopes, " ")},
		"response_mode":         {"query"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
// Microsoft OAuth tokens expire in 1 hour and require refresh.
func (h *OAuthHandler) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://login.microsoftonline.com/common/oauth2/v2.0/token",
		strings.NewReader(params.Encode()))
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

// RefreshToken refreshes an expired access token.
// Microsoft access tokens expire in 1 hour, so refresh is essential.
func (h *OAuthHandler) RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*driven.OAuthToken, error) {
	params := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://login.microsoftonline.com/common/oauth2/v2.0/token",
		strings.NewReader(params.Encode()))
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

// GetUserInfo fetches the authenticated user's information from Microsoft Graph.
func (h *OAuthHandler) GetUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
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
		ImageURL: "", // Microsoft Graph doesn't return avatar URL in /me endpoint
	}, nil
}

// DefaultConfig returns Microsoft's default OAuth configuration.
func (h *OAuthHandler) DefaultConfig() connectors.OAuthDefaults {
	return connectors.OAuthDefaults{
		AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:       []string{"Files.Read", "User.Read", "offline_access"},
		UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
		SupportsPKCE: true,
	}
}
