package driven

import (
	"context"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// TokenProvider provides access tokens for API authentication.
// It handles token storage, retrieval, and automatic refresh for OAuth.
type TokenProvider interface {
	// GetAccessToken returns a valid access token.
	// For OAuth, this automatically refreshes expired tokens.
	// For API keys, this returns the stored key.
	GetAccessToken(ctx context.Context) (string, error)

	// GetCredentials returns the full credentials.
	// Use GetAccessToken for most operations - this is for special cases.
	GetCredentials(ctx context.Context) (*domain.Credentials, error)

	// AuthMethod returns the authentication method.
	AuthMethod() domain.AuthMethod

	// IsValid checks if the credentials are still valid.
	IsValid(ctx context.Context) bool
}

// TokenProviderFactory creates TokenProviders from connection IDs.
// It resolves connection credentials and wraps them in appropriate TokenProviders.
type TokenProviderFactory interface {
	// Create creates a TokenProvider for a connection.
	// It looks up the connection by ID, decrypts credentials, and creates
	// an appropriate TokenProvider based on the auth method.
	Create(ctx context.Context, connectionID string) (TokenProvider, error)

	// CreateFromConnection creates a TokenProvider from a connection directly.
	// Use this when you already have the connection loaded.
	CreateFromConnection(ctx context.Context, conn *domain.Connection) (TokenProvider, error)
}

// TokenRefresher handles OAuth token refresh operations.
// This is used internally by OAuth TokenProviders.
type TokenRefresher interface {
	// Refresh refreshes the OAuth tokens.
	// Returns the new tokens and updates the credentials in the store.
	Refresh(ctx context.Context, creds *domain.Credentials) (*domain.Credentials, error)
}

// StaticTokenProvider implements TokenProvider for non-OAuth credentials.
// Used for API keys, PATs, and basic auth.
type StaticTokenProvider struct {
	creds *domain.Credentials
}

// NewStaticTokenProvider creates a token provider for static credentials.
func NewStaticTokenProvider(creds *domain.Credentials) *StaticTokenProvider {
	return &StaticTokenProvider{creds: creds}
}

// GetAccessToken returns the API key or PAT.
func (p *StaticTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	switch p.creds.AuthMethod {
	case domain.AuthMethodAPIKey:
		return p.creds.APIKey, nil
	case domain.AuthMethodPAT:
		return p.creds.APIKey, nil // PAT stored in APIKey field
	case domain.AuthMethodBasic:
		// For basic auth, return empty - caller should use GetCredentials
		return "", nil
	default:
		return "", nil
	}
}

// GetCredentials returns the full credentials.
func (p *StaticTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	return p.creds, nil
}

// AuthMethod returns the authentication method.
func (p *StaticTokenProvider) AuthMethod() domain.AuthMethod {
	return p.creds.AuthMethod
}

// IsValid returns true - static credentials don't expire.
func (p *StaticTokenProvider) IsValid(ctx context.Context) bool {
	return true
}

// OAuthTokenProvider implements TokenProvider for OAuth credentials.
// It automatically refreshes tokens when they expire.
type OAuthTokenProvider struct {
	creds      *domain.Credentials
	refresher  TokenRefresher
	store      CredentialsStore
	checkEvery time.Duration
}

// NewOAuthTokenProvider creates a token provider for OAuth credentials.
func NewOAuthTokenProvider(
	creds *domain.Credentials,
	refresher TokenRefresher,
	store CredentialsStore,
) *OAuthTokenProvider {
	return &OAuthTokenProvider{
		creds:      creds,
		refresher:  refresher,
		store:      store,
		checkEvery: 30 * time.Second, // Check freshness every 30s
	}
}

// GetAccessToken returns a valid access token, refreshing if needed.
func (p *OAuthTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	// Check if we need to refresh
	if p.creds.NeedsRefresh() {
		newCreds, err := p.refresher.Refresh(ctx, p.creds)
		if err != nil {
			return "", err
		}
		p.creds = newCreds
	}

	return p.creds.AccessToken, nil
}

// GetCredentials returns the full credentials.
func (p *OAuthTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	// Ensure tokens are fresh
	if p.creds.NeedsRefresh() {
		newCreds, err := p.refresher.Refresh(ctx, p.creds)
		if err != nil {
			return nil, err
		}
		p.creds = newCreds
	}
	return p.creds, nil
}

// AuthMethod returns OAuth2.
func (p *OAuthTokenProvider) AuthMethod() domain.AuthMethod {
	return domain.AuthMethodOAuth2
}

// IsValid checks if credentials are valid (not expired or can be refreshed).
func (p *OAuthTokenProvider) IsValid(ctx context.Context) bool {
	// If we have a refresh token, we can always refresh
	if p.creds.RefreshToken != "" {
		return true
	}
	// Otherwise, check if access token is still valid
	return !p.creds.IsExpired()
}
