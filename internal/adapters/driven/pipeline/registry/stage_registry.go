package registry

import (
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

// StageRegistry is an in-memory registry of stage factories.
type StageRegistry struct {
	mu        sync.RWMutex
	factories map[string]pipelineport.StageFactory
}

// NewStageRegistry creates a new stage registry.
func NewStageRegistry() *StageRegistry {
	return &StageRegistry{
		factories: make(map[string]pipelineport.StageFactory),
	}
}

// Register registers a stage factory.
func (r *StageRegistry) Register(factory pipelineport.StageFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := factory.StageID()
	if _, exists := r.factories[id]; exists {
		return fmt.Errorf("stage factory already registered: %s", id)
	}

	r.factories[id] = factory
	return nil
}

// Get retrieves a stage factory by ID.
func (r *StageRegistry) Get(stageID string) (pipelineport.StageFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[stageID]
	return factory, ok
}

// List returns descriptors for all registered stages.
func (r *StageRegistry) List() []pipeline.StageDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]pipeline.StageDescriptor, 0, len(r.factories))
	for _, factory := range r.factories {
		descriptors = append(descriptors, factory.Descriptor())
	}
	return descriptors
}

// ListByType returns descriptors for stages of a specific type.
func (r *StageRegistry) ListByType(stageType pipeline.StageType) []pipeline.StageDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var descriptors []pipeline.StageDescriptor
	for _, factory := range r.factories {
		desc := factory.Descriptor()
		if desc.Type == stageType {
			descriptors = append(descriptors, desc)
		}
	}
	return descriptors
}

// Ensure StageRegistry implements the interface.
var _ pipelineport.StageRegistry = (*StageRegistry)(nil)
