package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// OAuthService handles OAuth authentication flows for connector installations.
// It manages the OAuth authorization flow, token exchange, and installation creation.
type OAuthService interface {
	// Authorize starts an OAuth authorization flow.
	// Returns an authorization URL to redirect the user to.
	// The state parameter is stored for CSRF validation during callback.
	Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeResponse, error)

	// Callback handles the OAuth callback from the provider.
	// It exchanges the authorization code for tokens and creates an installation.
	// Returns the created installation summary.
	Callback(ctx context.Context, req CallbackRequest) (*CallbackResponse, error)
}

// AuthorizeRequest represents a request to start an OAuth flow.
// @Description Request to start OAuth authorization flow
type AuthorizeRequest struct {
	// ProviderType is the OAuth provider (github, slack, notion, etc.)
	ProviderType domain.ProviderType `json:"provider_type" example:"github"`

	// InstallationName is an optional name for the installation.
	// If not provided, defaults to "{Provider} ({AccountID})"
	InstallationName string `json:"installation_name,omitempty" example:"My GitHub"`
}

// AuthorizeResponse contains the authorization URL and state.
// @Description Response containing the OAuth authorization URL
type AuthorizeResponse struct {
	// AuthorizationURL is the URL to redirect the user to for authorization.
	AuthorizationURL string `json:"authorization_url" example:"https://github.com/login/oauth/authorize?client_id=..."`

	// State is the CSRF token that will be returned in the callback.
	// This is provided for reference - the frontend should not need to track it.
	State string `json:"state" example:"abc123xyz"`

	// ExpiresAt is when the authorization state expires (typically 10 minutes).
	ExpiresAt string `json:"expires_at" example:"2024-01-15T10:10:00Z"`
}

// CallbackRequest represents the OAuth callback from the provider.
// @Description OAuth callback parameters from provider redirect
type CallbackRequest struct {
	// Code is the authorization code from the provider.
	Code string `json:"code" example:"abc123"`

	// State is the CSRF token returned by the provider.
	State string `json:"state" example:"abc123xyz"`

	// Error is set if the provider returned an error.
	Error string `json:"error,omitempty" example:"access_denied"`

	// ErrorDescription provides details about the error.
	ErrorDescription string `json:"error_description,omitempty" example:"The user denied access"`
}

// CallbackResponse contains the result of the OAuth callback.
// @Description Response after successful OAuth authorization
type CallbackResponse struct {
	// Installation is the created installation summary.
	Installation *domain.ConnectionSummary `json:"installation"`

	// Message provides a human-readable status message.
	Message string `json:"message" example:"Successfully connected to GitHub as octocat"`
}

// OAuthError represents an OAuth-specific error.
type OAuthError struct {
	Code        string `json:"error" example:"invalid_state"`
	Description string `json:"error_description" example:"The state parameter is invalid or expired"`
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return e.Code + ": " + e.Description
	}
	return e.Code
}

// Common OAuth errors
var (
	ErrOAuthInvalidState     = &OAuthError{Code: "invalid_state", Description: "The state parameter is invalid or expired"}
	ErrOAuthProviderNotFound = &OAuthError{Code: "provider_not_found", Description: "The provider is not configured"}
	ErrOAuthProviderDisabled = &OAuthError{Code: "provider_disabled", Description: "The provider is not enabled"}
	ErrOAuthExchangeFailed   = &OAuthError{Code: "exchange_failed", Description: "Failed to exchange authorization code for tokens"}
	ErrOAuthUserInfoFailed   = &OAuthError{Code: "user_info_failed", Description: "Failed to fetch user information"}
)
