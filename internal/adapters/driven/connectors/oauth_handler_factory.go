package connectors

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure OAuthHandlerFactoryAdapter implements driven.OAuthHandlerFactory
var _ driven.OAuthHandlerFactory = (*OAuthHandlerFactoryAdapter)(nil)

// OAuthHandlerFactoryAdapter adapts connectors.Factory to implement driven.OAuthHandlerFactory.
// This abstraction allows services to avoid importing the adapters layer directly.
type OAuthHandlerFactoryAdapter struct {
	factory *Factory
}

// NewOAuthHandlerFactory creates an adapter that implements driven.OAuthHandlerFactory
func NewOAuthHandlerFactory(factory *Factory) *OAuthHandlerFactoryAdapter {
	return &OAuthHandlerFactoryAdapter{factory: factory}
}

// GetOAuthHandler implements driven.OAuthHandlerFactory
func (a *OAuthHandlerFactoryAdapter) GetOAuthHandler(providerType domain.ProviderType) driven.OAuthHandler {
	handler := a.factory.GetOAuthHandler(providerType)
	if handler == nil {
		return nil
	}
	return &oauthHandlerAdapter{handler: handler}
}

// oauthHandlerAdapter adapts connectors.OAuthHandler to driven.OAuthHandler
type oauthHandlerAdapter struct {
	handler OAuthHandler
}

// Ensure oauthHandlerAdapter implements driven.OAuthHandler
var _ driven.OAuthHandler = (*oauthHandlerAdapter)(nil)

// DefaultConfig implements driven.OAuthHandler
func (a *oauthHandlerAdapter) DefaultConfig() driven.OAuthConfig {
	defaults := a.handler.DefaultConfig()
	return driven.OAuthConfig{
		AuthURL:     defaults.AuthURL,
		TokenURL:    defaults.TokenURL,
		Scopes:      defaults.Scopes,
		UserInfoURL: defaults.UserInfoURL,
	}
}

// BuildAuthURL implements driven.OAuthHandler
func (a *oauthHandlerAdapter) BuildAuthURL(clientID, redirectURI, state, codeChallenge string, scopes []string) string {
	return a.handler.BuildAuthURL(clientID, redirectURI, state, codeChallenge, scopes)
}

// ExchangeCode implements driven.OAuthHandler
func (a *oauthHandlerAdapter) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*driven.OAuthToken, error) {
	return a.handler.ExchangeCode(ctx, clientID, clientSecret, code, redirectURI, codeVerifier)
}

// GetUserInfo implements driven.OAuthHandler
func (a *oauthHandlerAdapter) GetUserInfo(ctx context.Context, accessToken string) (*driven.OAuthUserInfo, error) {
	return a.handler.GetUserInfo(ctx, accessToken)
}

// RefreshToken implements driven.OAuthHandler
func (a *oauthHandlerAdapter) RefreshToken(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
	// Note: The connectors.OAuthHandler.RefreshToken requires clientID and clientSecret,
	// but the driven.OAuthHandler only takes refreshToken. This is a design mismatch.
	// For now, we'll return an error indicating this needs to be handled at a higher level.
	// The actual refresh logic is in TokenProviderFactory which has access to credentials.
	return nil, ErrRefreshNotSupported
}

// ErrRefreshNotSupported indicates refresh must be handled via TokenProviderFactory
var ErrRefreshNotSupported = domain.ErrUnsupportedProvider // Reuse existing error for now
