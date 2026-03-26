package pipeline

import "sync"

// CapabilitySet holds available capability providers for a pipeline run.
type CapabilitySet struct {
	mu        sync.RWMutex
	providers map[CapabilityType][]CapabilityInstance
}

// CapabilityInstance represents a resolved capability provider.
type CapabilityInstance struct {
	ID       string         `json:"id"`
	Type     CapabilityType `json:"type"`
	Instance any            `json:"-"` // The actual service (embedder, LLM client, etc.)
}

// NewCapabilitySet creates an empty capability set.
func NewCapabilitySet() *CapabilitySet {
	return &CapabilitySet{
		providers: make(map[CapabilityType][]CapabilityInstance),
	}
}

// Add adds a capability instance to the set.
func (cs *CapabilitySet) Add(capType CapabilityType, id string, instance any) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.providers[capType] = append(cs.providers[capType], CapabilityInstance{
		ID:       id,
		Type:     capType,
		Instance: instance,
	})
}

// Get retrieves the first capability instance of a given type.
func (cs *CapabilitySet) Get(capType CapabilityType) (CapabilityInstance, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	instances, ok := cs.providers[capType]
	if !ok || len(instances) == 0 {
		return CapabilityInstance{}, false
	}
	return instances[0], true
}

// GetByID retrieves a specific capability instance by type and ID.
func (cs *CapabilitySet) GetByID(capType CapabilityType, id string) (CapabilityInstance, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	instances, ok := cs.providers[capType]
	if !ok {
		return CapabilityInstance{}, false
	}

	for _, inst := range instances {
		if inst.ID == id {
			return inst, true
		}
	}
	return CapabilityInstance{}, false
}

// GetAll retrieves all capability instances of a given type.
func (cs *CapabilitySet) GetAll(capType CapabilityType) []CapabilityInstance {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	instances, ok := cs.providers[capType]
	if !ok {
		return nil
	}

	result := make([]CapabilityInstance, len(instances))
	copy(result, instances)
	return result
}

// Has checks if a capability type is available.
func (cs *CapabilitySet) Has(capType CapabilityType) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	instances, ok := cs.providers[capType]
	return ok && len(instances) > 0
}

// Types returns all capability types in the set.
func (cs *CapabilitySet) Types() []CapabilityType {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	types := make([]CapabilityType, 0, len(cs.providers))
	for t := range cs.providers {
		types = append(types, t)
	}
	return types
}
