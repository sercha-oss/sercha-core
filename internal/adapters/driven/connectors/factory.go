package connectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Factory implements the interface.
var _ driven.ConnectorFactory = (*Factory)(nil)

// Factory creates connectors and manages OAuth handlers.
// It maintains a registry of ConnectorBuilders and OAuthHandlers for each provider type.
type Factory struct {
	mu                   sync.RWMutex
	builders             map[domain.ProviderType]driven.ConnectorBuilder
	oauthHandlers        map[domain.PlatformType]OAuthHandler
	tokenProviderFactory driven.TokenProviderFactory
}

// NewFactory creates a connector factory.
func NewFactory(tokenProviderFactory driven.TokenProviderFactory) *Factory {
	return &Factory{
		builders:             make(map[domain.ProviderType]driven.ConnectorBuilder),
		oauthHandlers:        make(map[domain.PlatformType]OAuthHandler),
		tokenProviderFactory: tokenProviderFactory,
	}
}

// Register registers a connector builder for a provider type.
func (f *Factory) Register(builder driven.ConnectorBuilder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[builder.Type()] = builder
}

// RegisterOAuthHandler registers an OAuth handler for a platform type.
func (f *Factory) RegisterOAuthHandler(platform domain.PlatformType, handler OAuthHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oauthHandlers[platform] = handler
}

// Create creates a connector for the given source, scoped to a container.
// Called by SyncOrchestrator once per container in source.SelectedContainers.
// For providers without container selection, containerID may be empty.
//
// If source.ConnectionID is empty, no per-connection TokenProvider is
// resolved and the Builder is invoked with a nil token provider. This
// supports connectors whose credentials are deployment-level rather than
// per-connection (for example a Builder that captures static or
// service-principal credentials at registration time and ignores the
// argument). Builders that require a per-connection TokenProvider must
// either return an error from Build when given nil or document that a
// non-empty ConnectionID is required by setting Builder-level validation
// upstream of this call.
func (f *Factory) Create(ctx context.Context, source *domain.Source, containerID string) (driven.Connector, error) {
	f.mu.RLock()
	builder, ok := f.builders[source.ProviderType]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, source.ProviderType)
	}

	// Create token provider from connection. Skip the lookup when the
	// source has no ConnectionID — the Builder is responsible for either
	// supplying its own credentials or rejecting the nil provider.
	var tokenProvider driven.TokenProvider
	if source.ConnectionID != "" {
		tp, err := f.tokenProviderFactory.Create(ctx, source.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("create token provider: %w", err)
		}
		tokenProvider = tp
	}

	// Build connector scoped to container
	connector, err := builder.Build(ctx, tokenProvider, containerID)
	if err != nil {
		return nil, fmt.Errorf("build connector: %w", err)
	}

	return connector, nil
}

// SupportedTypes returns all registered provider types.
func (f *Factory) SupportedTypes() []domain.ProviderType {
	f.mu.RLock()
	defer f.mu.RUnlock()
	types := make([]domain.ProviderType, 0, len(f.builders))
	for t := range f.builders {
		types = append(types, t)
	}
	return types
}

// GetBuilder returns the builder for a provider type.
func (f *Factory) GetBuilder(providerType domain.ProviderType) (driven.ConnectorBuilder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	builder, ok := f.builders[providerType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, providerType)
	}
	return builder, nil
}

// SupportsOAuth returns true if the provider supports OAuth authentication.
func (f *Factory) SupportsOAuth(providerType domain.ProviderType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	builder, ok := f.builders[providerType]
	if !ok {
		return false
	}
	return builder.SupportsOAuth()
}

// GetOAuthConfig returns OAuth configuration for a provider.
// Returns nil if the provider doesn't support OAuth.
func (f *Factory) GetOAuthConfig(providerType domain.ProviderType) *driven.OAuthConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	builder, ok := f.builders[providerType]
	if !ok {
		return nil
	}
	return builder.OAuthConfig()
}

// GetOAuthHandler returns the OAuth handler for a platform type.
// Returns nil if no handler is registered.
func (f *Factory) GetOAuthHandler(platform domain.PlatformType) OAuthHandler {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.oauthHandlers[platform]
}

// SupportsContainerSelection returns true if the provider supports container selection.
func (f *Factory) SupportsContainerSelection(providerType domain.ProviderType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	builder, ok := f.builders[providerType]
	if !ok {
		return false
	}
	return builder.SupportsContainerSelection()
}
