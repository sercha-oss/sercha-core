package connectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure Factory implements the interface.
var _ driven.ConnectorFactory = (*Factory)(nil)

// Factory creates connectors and manages OAuth handlers.
// It maintains a registry of ConnectorBuilders and OAuthHandlers for each provider type.
type Factory struct {
	mu                   sync.RWMutex
	builders             map[domain.ProviderType]driven.ConnectorBuilder
	oauthHandlers        map[domain.ProviderType]OAuthHandler
	tokenProviderFactory driven.TokenProviderFactory
}

// NewFactory creates a connector factory.
func NewFactory(tokenProviderFactory driven.TokenProviderFactory) *Factory {
	return &Factory{
		builders:             make(map[domain.ProviderType]driven.ConnectorBuilder),
		oauthHandlers:        make(map[domain.ProviderType]OAuthHandler),
		tokenProviderFactory: tokenProviderFactory,
	}
}

// Register registers a connector builder for a provider type.
func (f *Factory) Register(builder driven.ConnectorBuilder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[builder.Type()] = builder
}

// RegisterOAuthHandler registers an OAuth handler for a provider type.
func (f *Factory) RegisterOAuthHandler(providerType domain.ProviderType, handler OAuthHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oauthHandlers[providerType] = handler
}

// Create creates a connector for the given source, scoped to a container.
// Called by SyncOrchestrator once per container in source.SelectedContainers.
// For providers without container selection, containerID may be empty.
func (f *Factory) Create(ctx context.Context, source *domain.Source, containerID string) (driven.Connector, error) {
	f.mu.RLock()
	builder, ok := f.builders[source.ProviderType]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, source.ProviderType)
	}

	// Create token provider from connection
	tokenProvider, err := f.tokenProviderFactory.Create(ctx, source.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("create token provider: %w", err)
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

// GetOAuthHandler returns the OAuth handler for a provider type.
// Returns nil if no handler is registered.
func (f *Factory) GetOAuthHandler(providerType domain.ProviderType) OAuthHandler {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.oauthHandlers[providerType]
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
