package connectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure ContainerListerFactoryImpl implements the interface.
var _ driven.ContainerListerFactory = (*ContainerListerFactoryImpl)(nil)

// ProviderContainerListerFactory creates ContainerListers for a specific provider type.
type ProviderContainerListerFactory interface {
	// Create creates a ContainerLister for a connection.
	Create(ctx context.Context, connectionID string) (driven.ContainerLister, error)
}

// ContainerListerFactoryImpl creates ContainerListers for different providers.
type ContainerListerFactoryImpl struct {
	mu        sync.RWMutex
	factories map[domain.ProviderType]ProviderContainerListerFactory
}

// NewContainerListerFactory creates a new container lister factory.
func NewContainerListerFactory() *ContainerListerFactoryImpl {
	return &ContainerListerFactoryImpl{
		factories: make(map[domain.ProviderType]ProviderContainerListerFactory),
	}
}

// Register registers a provider-specific factory.
func (f *ContainerListerFactoryImpl) Register(providerType domain.ProviderType, factory ProviderContainerListerFactory) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.factories[providerType] = factory
}

// Create creates a ContainerLister for the given provider and connection.
func (f *ContainerListerFactoryImpl) Create(ctx context.Context, providerType domain.ProviderType, connectionID string) (driven.ContainerLister, error) {
	f.mu.RLock()
	factory, ok := f.factories[providerType]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s does not support container selection", domain.ErrUnsupportedProvider, providerType)
	}

	return factory.Create(ctx, connectionID)
}

// SupportsContainerSelection returns true if the provider supports container selection.
func (f *ContainerListerFactoryImpl) SupportsContainerSelection(providerType domain.ProviderType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.factories[providerType]
	return ok
}
