package driving

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// OAuthServerService implements the OAuth 2.0 Authorization Server for MCP authentication.
// It handles client registration, authorization code flow with PKCE, token issuance,
// and token validation for bearer token authentication.
type OAuthServerService interface {
	// RegisterClient handles dynamic client registration (RFC 7591).
	// It validates the request, generates credentials, and returns the client info.
	// The client_secret is only returned in the response (not stored in plaintext).
	RegisterClient(ctx context.Context, req domain.ClientRegistrationRequest) (*domain.ClientRegistrationResponse, error)

	// Authorize processes an authorization request and returns an authorization code.
	// The caller (HTTP handler) is responsible for redirecting the user.
	// The userID comes from the authenticated session context.
	// PKCE (code_challenge with S256 method) is required per MCP specification.
	Authorize(ctx context.Context, userID string, req domain.AuthorizeRequest) (code string, err error)

	// Token exchanges an authorization code or refresh token for access/refresh tokens.
	// Supports two grant types:
	// - authorization_code: exchanges code + PKCE verifier for tokens
	// - refresh_token: exchanges refresh token for new tokens (with rotation)
	Token(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error)

	// Revoke revokes an access or refresh token (RFC 7009).
	// Always returns success per spec, even if token doesn't exist.
	Revoke(ctx context.Context, req domain.RevokeRequest) error

	// ValidateAccessToken validates a bearer token and returns token info.
	// Used by the MCP server's bearer token middleware to authenticate requests.
	// Validates JWT signature, expiration, revocation status, and audience.
	ValidateAccessToken(ctx context.Context, token string) (*OAuthTokenInfo, error)

	// GetServerMetadata returns OAuth Authorization Server Metadata (RFC 8414).
	// The baseURL parameter is used to construct absolute endpoint URLs.
	GetServerMetadata(baseURL string) *OAuthServerMetadata
}

// OAuthTokenInfo contains validated token information for middleware use
type OAuthTokenInfo struct {
	UserID   string   // Subject (user ID) from token
	ClientID string   // OAuth client ID that requested the token
	Scopes   []string // Granted scopes
	Audience string   // Resource indicator (MCP server URL)
}

// OAuthServerMetadata represents RFC 8414 Authorization Server Metadata
type OAuthServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResourceIndicatorsSupported       bool     `json:"resource_indicators_supported,omitempty"` // For client discovery
}
