package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// OAuthClientStore manages OAuth 2.0 client registrations
type OAuthClientStore interface {
	// Save stores a new client or updates an existing one
	Save(ctx context.Context, client *domain.OAuthClient) error

	// Get retrieves a client by client_id
	// Returns domain.ErrNotFound if the client doesn't exist
	Get(ctx context.Context, clientID string) (*domain.OAuthClient, error)

	// Delete removes a client by client_id
	// Returns domain.ErrNotFound if the client doesn't exist
	Delete(ctx context.Context, clientID string) error

	// List retrieves all registered clients
	List(ctx context.Context) ([]*domain.OAuthClient, error)
}

// AuthorizationCodeStore manages short-lived authorization codes for the auth code flow
type AuthorizationCodeStore interface {
	// Save stores a new authorization code
	Save(ctx context.Context, code *domain.AuthorizationCode) error

	// GetAndMarkUsed atomically retrieves the code and marks it as used.
	// This ensures single-use semantics for authorization codes.
	// Returns domain.ErrNotFound if code doesn't exist.
	GetAndMarkUsed(ctx context.Context, code string) (*domain.AuthorizationCode, error)

	// Cleanup removes expired authorization codes.
	// Should be called periodically (e.g., via cron job).
	Cleanup(ctx context.Context) error
}

// OAuthTokenStore manages access tokens and refresh tokens with revocation support
type OAuthTokenStore interface {
	// SaveAccessToken stores a new access token
	SaveAccessToken(ctx context.Context, token *domain.OAuthAccessToken) error

	// GetAccessToken retrieves an access token by its ID (jti claim)
	// Returns domain.ErrNotFound if the token doesn't exist
	GetAccessToken(ctx context.Context, tokenID string) (*domain.OAuthAccessToken, error)

	// RevokeAccessToken marks an access token as revoked
	// Returns domain.ErrNotFound if the token doesn't exist
	RevokeAccessToken(ctx context.Context, tokenID string) error

	// SaveRefreshToken stores a new refresh token
	SaveRefreshToken(ctx context.Context, token *domain.OAuthRefreshToken) error

	// GetRefreshToken retrieves a refresh token by its ID
	// Returns domain.ErrNotFound if the token doesn't exist
	GetRefreshToken(ctx context.Context, tokenID string) (*domain.OAuthRefreshToken, error)

	// RevokeRefreshToken marks a refresh token as revoked
	// Returns domain.ErrNotFound if the token doesn't exist
	RevokeRefreshToken(ctx context.Context, tokenID string) error

	// RotateRefreshToken marks the old token as rotated and returns the new token ID.
	// This implements refresh token rotation for enhanced security.
	// Returns domain.ErrNotFound if the old token doesn't exist.
	RotateRefreshToken(ctx context.Context, oldTokenID string, newTokenID string) error

	// RevokeAllForClient revokes all tokens (access and refresh) for a given client.
	// Used when a client is deleted or compromised.
	RevokeAllForClient(ctx context.Context, clientID string) error

	// Cleanup removes expired tokens.
	// Should be called periodically (e.g., via cron job).
	Cleanup(ctx context.Context) error
}
