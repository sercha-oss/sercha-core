package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// ScopeList is a []string that can unmarshal from either a JSON string
// (space-delimited, per RFC 7591) or a JSON array of strings.
type ScopeList []string

func (s ScopeList) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.Join(s, " "))
}

func (s *ScopeList) UnmarshalJSON(data []byte) error {
	// Try space-delimited string first (RFC standard)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			*s = []string{}
		} else {
			*s = strings.Fields(str)
		}
		return nil
	}
	// Also accept array for flexibility
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*s = arr
	return nil
}

// OAuthGrantType represents the OAuth 2.0 grant type
type OAuthGrantType string

const (
	GrantTypeAuthorizationCode OAuthGrantType = "authorization_code"
	GrantTypeRefreshToken      OAuthGrantType = "refresh_token"
)

// OAuthClient represents a registered third-party OAuth 2.0 client application
type OAuthClient struct {
	ID                      string           `json:"id"`                         // client_id (UUID)
	SecretHash              string           `json:"-"`                          // bcrypt hash of client_secret (empty for public clients)
	Name                    string           `json:"name"`                       // Human-readable name
	RedirectURIs            []string         `json:"redirect_uris"`              // Registered redirect URIs
	GrantTypes              []OAuthGrantType `json:"grant_types"`                // Allowed grant types
	ResponseTypes           []string         `json:"response_types"`             // Allowed response types (e.g., "code")
	Scopes                  []string         `json:"scopes"`                     // Allowed scopes
	ApplicationType         string           `json:"application_type"`           // "native" or "web"
	TokenEndpointAuthMethod string           `json:"token_endpoint_auth_method"` // "client_secret_post" or "none" (public)
	Active                  bool             `json:"active"`                     // Whether the client is active
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
}

// IsPublic returns true if the client is a public client (no client secret required)
func (c *OAuthClient) IsPublic() bool {
	return c.TokenEndpointAuthMethod == "none"
}

// HasRedirectURI validates if the given URI is in the client's registered redirect URIs
func (c *OAuthClient) HasRedirectURI(uri string) bool {
	for _, registered := range c.RedirectURIs {
		if registered == uri {
			return true
		}
	}
	return false
}

// HasGrantType checks if the client is allowed to use the given grant type
func (c *OAuthClient) HasGrantType(gt OAuthGrantType) bool {
	for _, allowed := range c.GrantTypes {
		if allowed == gt {
			return true
		}
	}
	return false
}

// HasScope checks if the client is allowed to request the given scope
func (c *OAuthClient) HasScope(scope string) bool {
	for _, allowed := range c.Scopes {
		if allowed == scope {
			return true
		}
	}
	return false
}

// ValidateScopes returns a list of invalid scopes from the requested list
func (c *OAuthClient) ValidateScopes(requested []string) []string {
	invalid := []string{}
	for _, scope := range requested {
		if !c.HasScope(scope) {
			invalid = append(invalid, scope)
		}
	}
	return invalid
}

// AuthorizationCode represents a short-lived authorization code for the auth code flow
type AuthorizationCode struct {
	Code          string    `json:"code"`
	ClientID      string    `json:"client_id"`
	UserID        string    `json:"user_id"`
	RedirectURI   string    `json:"redirect_uri"`
	Scopes        []string  `json:"scopes"`
	CodeChallenge string    `json:"code_challenge"` // PKCE S256 challenge
	Resource      string    `json:"resource"`       // RFC 8707 resource indicator (audience)
	ExpiresAt     time.Time `json:"expires_at"`     // 10 minutes
	Used          bool      `json:"used"`           // single-use flag
	CreatedAt     time.Time `json:"created_at"`
}

// IsExpired checks if the authorization code has expired
func (a *AuthorizationCode) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// IsUsable checks if the authorization code can be used (not expired and not already used)
func (a *AuthorizationCode) IsUsable() bool {
	return !a.IsExpired() && !a.Used
}

// OAuthAccessToken represents an OAuth 2.0 access token
type OAuthAccessToken struct {
	ID        string    `json:"id"` // jti claim
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	Audience  string    `json:"audience"`   // Resource indicator (MCP server URL)
	ExpiresAt time.Time `json:"expires_at"` // Short-lived (15 min default)
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

// IsExpired checks if the access token has expired
func (t *OAuthAccessToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the access token is valid (not expired and not revoked)
func (t *OAuthAccessToken) IsValid() bool {
	return !t.IsExpired() && !t.Revoked
}

// OAuthRefreshToken represents an OAuth 2.0 refresh token with rotation support
type OAuthRefreshToken struct {
	ID            string    `json:"id"`
	AccessTokenID string    `json:"access_token_id"` // Associated access token
	ClientID      string    `json:"client_id"`
	UserID        string    `json:"user_id"`
	Scopes        []string  `json:"scopes"`
	Audience      string    `json:"audience"`
	ExpiresAt     time.Time `json:"expires_at"` // 30 days
	CreatedAt     time.Time `json:"created_at"`
	Revoked       bool      `json:"revoked"`
	RotatedTo     string    `json:"rotated_to"` // New token ID after rotation (empty if current)
}

// IsExpired checks if the refresh token has expired
func (t *OAuthRefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the refresh token is valid (not expired, not revoked, and not rotated)
func (t *OAuthRefreshToken) IsValid() bool {
	return !t.IsExpired() && !t.Revoked && !t.IsRotated()
}

// IsRotated checks if the refresh token has been rotated to a new token
func (t *OAuthRefreshToken) IsRotated() bool {
	return t.RotatedTo != ""
}

// MCP Scope constants
const (
	ScopeMCPSearch      = "mcp:search"
	ScopeMCPDocRead     = "mcp:documents:read"
	ScopeMCPSourcesList = "mcp:sources:list"
)

// DefaultMCPScopes contains the default scopes for MCP clients
var DefaultMCPScopes = []string{ScopeMCPSearch, ScopeMCPDocRead, ScopeMCPSourcesList}

// Token configuration constants
const (
	AccessTokenTTL       = 15 * time.Minute
	RefreshTokenTTL      = 30 * 24 * time.Hour // 30 days
	AuthorizationCodeTTL = 10 * time.Minute
)

// ClientRegistrationRequest represents a dynamic client registration request (RFC 7591)
type ClientRegistrationRequest struct {
	Name                    string           `json:"client_name"`
	RedirectURIs            []string         `json:"redirect_uris"`
	GrantTypes              []OAuthGrantType `json:"grant_types,omitempty"`
	ResponseTypes           []string         `json:"response_types,omitempty"`
	Scopes                  ScopeList        `json:"scope,omitempty"` // Space-delimited string or array
	ApplicationType         string           `json:"application_type,omitempty"`
	TokenEndpointAuthMethod string           `json:"token_endpoint_auth_method,omitempty"`
}

// ClientRegistrationResponse represents the response from dynamic client registration
type ClientRegistrationResponse struct {
	ClientID                string           `json:"client_id"`
	ClientSecret            string           `json:"client_secret,omitempty"` // Only returned once for confidential clients
	Name                    string           `json:"client_name"`
	RedirectURIs            []string         `json:"redirect_uris"`
	GrantTypes              []OAuthGrantType `json:"grant_types"`
	ResponseTypes           []string         `json:"response_types"`
	Scopes                  ScopeList        `json:"scope"`
	ApplicationType         string           `json:"application_type"`
	TokenEndpointAuthMethod string           `json:"token_endpoint_auth_method"`
	ClientIDIssuedAt        int64            `json:"client_id_issued_at"` // Unix timestamp
}

// AuthorizeRequest represents an OAuth 2.0 authorization request
type AuthorizeRequest struct {
	ResponseType        string `json:"response_type"` // "code"
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`                 // Space-delimited scopes
	State               string `json:"state,omitempty"`       // CSRF token
	CodeChallenge       string `json:"code_challenge"`        // PKCE challenge
	CodeChallengeMethod string `json:"code_challenge_method"` // "S256"
	Resource            string `json:"resource,omitempty"`    // RFC 8707 resource indicator
}

// ParseScopes converts space-delimited scope string to array
func (r *AuthorizeRequest) ParseScopes() []string {
	if r.Scope == "" {
		return []string{}
	}
	return strings.Fields(r.Scope)
}

// TokenRequest represents an OAuth 2.0 token request
type TokenRequest struct {
	GrantType    OAuthGrantType `json:"grant_type"`
	Code         string         `json:"code,omitempty"`         // For authorization_code grant
	RedirectURI  string         `json:"redirect_uri,omitempty"` // For authorization_code grant
	ClientID     string         `json:"client_id"`
	ClientSecret string         `json:"client_secret,omitempty"` // For confidential clients
	CodeVerifier string         `json:"code_verifier,omitempty"` // PKCE verifier
	RefreshToken string         `json:"refresh_token,omitempty"` // For refresh_token grant
	Resource     string         `json:"resource,omitempty"`      // RFC 8707 resource indicator
}

// TokenResponse represents an OAuth 2.0 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`              // "Bearer"
	ExpiresIn    int64  `json:"expires_in"`              // Seconds until expiration
	RefreshToken string `json:"refresh_token,omitempty"` // Optional refresh token
	Scope        string `json:"scope,omitempty"`         // Space-delimited scopes
}

// RevokeRequest represents a token revocation request (RFC 7009)
type RevokeRequest struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"token_type_hint,omitempty"` // "access_token" or "refresh_token"
}
