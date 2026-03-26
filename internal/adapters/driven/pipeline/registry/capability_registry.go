package registry

import (
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

// CapabilityRegistry is an in-memory registry of capability providers.
type CapabilityRegistry struct {
	mu        sync.RWMutex
	providers map[pipeline.CapabilityType]map[string]pipelineport.CapabilityProvider
	defaults  map[pipeline.CapabilityType]string
}

// NewCapabilityRegistry creates a new capability registry.
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		providers: make(map[pipeline.CapabilityType]map[string]pipelineport.CapabilityProvider),
		defaults:  make(map[pipeline.CapabilityType]string),
	}
}

// Register registers a capability provider.
func (r *CapabilityRegistry) Register(provider pipelineport.CapabilityProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	capType := provider.Type()
	id := provider.ID()

	if r.providers[capType] == nil {
		r.providers[capType] = make(map[string]pipelineport.CapabilityProvider)
	}

	r.providers[capType][id] = provider

	// Set as default if it's the first of its type
	if _, hasDefault := r.defaults[capType]; !hasDefault {
		r.defaults[capType] = id
	}

	return nil
}

// Unregister removes a capability provider.
func (r *CapabilityRegistry) Unregister(capType pipeline.CapabilityType, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.providers[capType] == nil {
		return fmt.Errorf("no providers registered for type: %s", capType)
	}

	if _, ok := r.providers[capType][id]; !ok {
		return fmt.Errorf("provider not found: %s/%s", capType, id)
	}

	delete(r.providers[capType], id)

	// Clear default if it was this provider
	if r.defaults[capType] == id {
		delete(r.defaults, capType)
		// Set new default to first available
		for newID := range r.providers[capType] {
			r.defaults[capType] = newID
			break
		}
	}

	return nil
}

// Get retrieves a specific capability provider.
func (r *CapabilityRegistry) Get(capType pipeline.CapabilityType, id string) (pipelineport.CapabilityProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.providers[capType] == nil {
		return nil, false
	}

	provider, ok := r.providers[capType][id]
	return provider, ok
}

// GetDefault retrieves the default provider for a capability type.
func (r *CapabilityRegistry) GetDefault(capType pipeline.CapabilityType) (pipelineport.CapabilityProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.defaults[capType]
	if !ok {
		return nil, false
	}

	provider, ok := r.providers[capType][id]
	return provider, ok
}

// SetDefault sets the default provider for a capability type.
func (r *CapabilityRegistry) SetDefault(capType pipeline.CapabilityType, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.providers[capType] == nil {
		return fmt.Errorf("no providers registered for type: %s", capType)
	}

	if _, ok := r.providers[capType][id]; !ok {
		return fmt.Errorf("provider not found: %s/%s", capType, id)
	}

	r.defaults[capType] = id
	return nil
}

// List returns all providers of a capability type.
func (r *CapabilityRegistry) List(capType pipeline.CapabilityType) []pipelineport.CapabilityProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.providers[capType] == nil {
		return nil
	}

	providers := make([]pipelineport.CapabilityProvider, 0, len(r.providers[capType]))
	for _, provider := range r.providers[capType] {
		providers = append(providers, provider)
	}
	return providers
}

// ListAvailable returns all currently available providers of a type.
func (r *CapabilityRegistry) ListAvailable(capType pipeline.CapabilityType) []pipelineport.CapabilityProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.providers[capType] == nil {
		return nil
	}

	var providers []pipelineport.CapabilityProvider
	for _, provider := range r.providers[capType] {
		if provider.Available() {
			providers = append(providers, provider)
		}
	}
	return providers
}

// BuildCapabilitySet builds a CapabilitySet from registered providers.
func (r *CapabilityRegistry) BuildCapabilitySet(required []pipeline.CapabilityRequirement) (*pipeline.CapabilitySet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cs := pipeline.NewCapabilitySet()

	for _, req := range required {
		provider, ok := r.GetDefaultLocked(req.Type)
		if !ok {
			if req.Mode == pipeline.CapabilityRequired {
				return nil, fmt.Errorf("required capability not available: %s", req.Type)
			}
			continue
		}

		if !provider.Available() {
			if req.Mode == pipeline.CapabilityRequired {
				return nil, fmt.Errorf("required capability not available: %s", req.Type)
			}
			continue
		}

		cs.Add(req.Type, provider.ID(), provider.Instance())
	}

	return cs, nil
}

// GetDefaultLocked retrieves the default provider without acquiring lock.
// Caller must hold at least a read lock.
func (r *CapabilityRegistry) GetDefaultLocked(capType pipeline.CapabilityType) (pipelineport.CapabilityProvider, bool) {
	id, ok := r.defaults[capType]
	if !ok {
		return nil, false
	}

	provider, ok := r.providers[capType][id]
	return provider, ok
}

// Ensure CapabilityRegistry implements the interface.
var _ pipelineport.CapabilityRegistry = (*CapabilityRegistry)(nil)
