package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure TokenProviderFactory implements the interface.
var _ driven.TokenProviderFactory = (*TokenProviderFactory)(nil)

// TokenRefresherFunc is a function type for token refresh operations.
type TokenRefresherFunc func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error)

// TokenProviderFactory creates TokenProviders from connection credentials.
type TokenProviderFactory struct {
	connectionStore driven.ConnectionStore
	refreshers      map[domain.ProviderType]TokenRefresherFunc
}

// NewTokenProviderFactory creates a new TokenProviderFactory.
func NewTokenProviderFactory(
	connectionStore driven.ConnectionStore,
) *TokenProviderFactory {
	return &TokenProviderFactory{
		connectionStore: connectionStore,
		refreshers:      make(map[domain.ProviderType]TokenRefresherFunc),
	}
}

// RegisterRefresher registers a token refresh function for a provider type.
func (f *TokenProviderFactory) RegisterRefresher(
	providerType domain.ProviderType,
	refresher TokenRefresherFunc,
) {
	f.refreshers[providerType] = refresher
}

// Create creates a TokenProvider for a connection.
// It looks up the connection by ID, decrypts credentials, and creates
// an appropriate TokenProvider based on the auth method.
func (f *TokenProviderFactory) Create(ctx context.Context, connectionID string) (driven.TokenProvider, error) {
	conn, err := f.connectionStore.Get(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if conn == nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrConnectionNotFound, connectionID)
	}

	return f.CreateFromConnection(ctx, conn)
}

// CreateFromConnection creates a TokenProvider from a connection directly.
// Use this when you already have the connection loaded.
func (f *TokenProviderFactory) CreateFromConnection(ctx context.Context, conn *domain.Connection) (driven.TokenProvider, error) {
	if conn.Secrets == nil {
		return nil, fmt.Errorf("connection has no secrets: %s", conn.ID)
	}

	switch conn.AuthMethod {
	case domain.AuthMethodOAuth2:
		refresher := f.refreshers[conn.ProviderType]
		return NewOAuthTokenProvider(
			conn.ID,
			conn.Secrets.AccessToken,
			conn.Secrets.RefreshToken,
			conn.OAuthExpiry,
			refresher,
			f.connectionStore,
		), nil

	case domain.AuthMethodAPIKey:
		return NewStaticTokenProvider(conn.Secrets.APIKey, domain.AuthMethodAPIKey), nil

	case domain.AuthMethodPAT:
		token := conn.Secrets.APIKey
		if token == "" {
			token = conn.Secrets.AccessToken
		}
		return NewStaticTokenProvider(token, domain.AuthMethodPAT), nil

	case domain.AuthMethodServiceAccount:
		// Service accounts typically use the service account JSON as-is
		return NewStaticTokenProvider(conn.Secrets.ServiceAccountJSON, domain.AuthMethodServiceAccount), nil

	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedAuthMethod, conn.AuthMethod)
	}
}

// StaticTokenProvider implements TokenProvider for non-OAuth credentials.
// Used for API keys, PATs, and service accounts.
type StaticTokenProvider struct {
	token      string
	authMethod domain.AuthMethod
}

// NewStaticTokenProvider creates a token provider for static credentials.
func NewStaticTokenProvider(token string, authMethod domain.AuthMethod) *StaticTokenProvider {
	return &StaticTokenProvider{
		token:      token,
		authMethod: authMethod,
	}
}

// GetAccessToken returns the static token.
func (p *StaticTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	return p.token, nil
}

// GetCredentials returns nil for static tokens - use GetAccessToken instead.
func (p *StaticTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	return &domain.Credentials{
		AuthMethod: p.authMethod,
		APIKey:     p.token,
	}, nil
}

// AuthMethod returns the authentication method.
func (p *StaticTokenProvider) AuthMethod() domain.AuthMethod {
	return p.authMethod
}

// IsValid returns true - static credentials don't expire.
func (p *StaticTokenProvider) IsValid(ctx context.Context) bool {
	return true
}

// OAuthTokenProvider implements TokenProvider for OAuth2 credentials.
// It automatically refreshes tokens when they expire.
type OAuthTokenProvider struct {
	connectionID    string
	accessToken     string
	refreshToken    string
	expiry          *time.Time
	refresher       TokenRefresherFunc
	connectionStore driven.ConnectionStore
}

// NewOAuthTokenProvider creates a token provider for OAuth credentials.
func NewOAuthTokenProvider(
	connectionID string,
	accessToken string,
	refreshToken string,
	expiry *time.Time,
	refresher TokenRefresherFunc,
	connectionStore driven.ConnectionStore,
) *OAuthTokenProvider {
	return &OAuthTokenProvider{
		connectionID:    connectionID,
		accessToken:     accessToken,
		refreshToken:    refreshToken,
		expiry:          expiry,
		refresher:       refresher,
		connectionStore: connectionStore,
	}
}

// GetAccessToken returns a valid access token, refreshing if needed.
func (p *OAuthTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	// Check if we need to refresh
	if p.needsRefresh() {
		if err := p.refresh(ctx); err != nil {
			return "", fmt.Errorf("refresh token: %w", err)
		}
	}
	return p.accessToken, nil
}

// GetCredentials returns credentials for OAuth.
func (p *OAuthTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	if p.needsRefresh() {
		if err := p.refresh(ctx); err != nil {
			return nil, fmt.Errorf("refresh token: %w", err)
		}
	}
	return &domain.Credentials{
		AuthMethod:   domain.AuthMethodOAuth2,
		AccessToken:  p.accessToken,
		RefreshToken: p.refreshToken,
		TokenExpiry:  p.expiry,
	}, nil
}

// AuthMethod returns OAuth2.
func (p *OAuthTokenProvider) AuthMethod() domain.AuthMethod {
	return domain.AuthMethodOAuth2
}

// IsValid checks if credentials are valid (not expired or can be refreshed).
func (p *OAuthTokenProvider) IsValid(ctx context.Context) bool {
	// If we have a refresh token and refresher, we can always refresh
	if p.refreshToken != "" && p.refresher != nil {
		return true
	}
	// Otherwise, check if access token is still valid
	return !p.isExpired()
}

// needsRefresh returns true if the token should be refreshed.
func (p *OAuthTokenProvider) needsRefresh() bool {
	if p.expiry == nil {
		return false
	}
	// Refresh if expiring within 5 minutes
	return time.Until(*p.expiry) < 5*time.Minute
}

// isExpired returns true if the token has expired.
func (p *OAuthTokenProvider) isExpired() bool {
	if p.expiry == nil {
		return false
	}
	return time.Now().After(*p.expiry)
}

// refresh refreshes the access token using the refresh token.
func (p *OAuthTokenProvider) refresh(ctx context.Context) error {
	if p.refresher == nil {
		return fmt.Errorf("no token refresher configured")
	}
	if p.refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	tokens, err := p.refresher(ctx, p.refreshToken)
	if err != nil {
		return err
	}

	// Update local state
	p.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		p.refreshToken = tokens.RefreshToken
	}
	if tokens.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
		p.expiry = &expiry
	}

	// Update connection store
	if p.connectionStore != nil {
		secrets := &domain.ConnectionSecrets{
			AccessToken:  p.accessToken,
			RefreshToken: p.refreshToken,
		}
		// Ignore error - we have the tokens locally, persistence failure is non-fatal
		_ = p.connectionStore.UpdateSecrets(ctx, p.connectionID, secrets, p.expiry)
	}

	return nil
}
