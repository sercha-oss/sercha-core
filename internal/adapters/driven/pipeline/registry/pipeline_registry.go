package registry

import (
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

// PipelineRegistry is an in-memory registry of pipeline definitions.
type PipelineRegistry struct {
	mu        sync.RWMutex
	pipelines map[string]pipeline.PipelineDefinition
	defaults  map[pipeline.PipelineType]string
}

// NewPipelineRegistry creates a new pipeline registry.
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{
		pipelines: make(map[string]pipeline.PipelineDefinition),
		defaults:  make(map[pipeline.PipelineType]string),
	}
}

// Register registers a pipeline definition.
func (r *PipelineRegistry) Register(def pipeline.PipelineDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.pipelines[def.ID]; exists {
		return fmt.Errorf("pipeline already registered: %s", def.ID)
	}

	r.pipelines[def.ID] = def

	// Set as default if it's the first of its type
	if _, hasDefault := r.defaults[def.Type]; !hasDefault {
		r.defaults[def.Type] = def.ID
	}

	return nil
}

// Get retrieves a pipeline definition by ID.
func (r *PipelineRegistry) Get(pipelineID string) (pipeline.PipelineDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.pipelines[pipelineID]
	return def, ok
}

// List returns all registered pipeline definitions.
func (r *PipelineRegistry) List() []pipeline.PipelineDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]pipeline.PipelineDefinition, 0, len(r.pipelines))
	for _, def := range r.pipelines {
		defs = append(defs, def)
	}
	return defs
}

// ListByType returns pipeline definitions of a specific type.
func (r *PipelineRegistry) ListByType(pipelineType pipeline.PipelineType) []pipeline.PipelineDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []pipeline.PipelineDefinition
	for _, def := range r.pipelines {
		if def.Type == pipelineType {
			defs = append(defs, def)
		}
	}
	return defs
}

// GetDefault returns the default pipeline for a given type.
func (r *PipelineRegistry) GetDefault(pipelineType pipeline.PipelineType) (pipeline.PipelineDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.defaults[pipelineType]
	if !ok {
		return pipeline.PipelineDefinition{}, false
	}

	def, ok := r.pipelines[id]
	return def, ok
}

// SetDefault sets the default pipeline for a given type.
func (r *PipelineRegistry) SetDefault(pipelineType pipeline.PipelineType, pipelineID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	def, ok := r.pipelines[pipelineID]
	if !ok {
		return fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	if def.Type != pipelineType {
		return fmt.Errorf("pipeline %s is type %s, not %s", pipelineID, def.Type, pipelineType)
	}

	r.defaults[pipelineType] = pipelineID
	return nil
}

// Ensure PipelineRegistry implements the interface.
var _ pipelineport.PipelineRegistry = (*PipelineRegistry)(nil)
