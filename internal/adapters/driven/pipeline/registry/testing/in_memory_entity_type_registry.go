// Package registrytesting provides test-only in-memory implementations of
// pipeline registry interfaces. None of these types are production adapters;
// they exist solely to support unit and integration tests that need a
// controllable, dependency-free substitute for a real backing store.
//
// # Import path
//
//	import registrytesting "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/registry/testing"
//
// Test files that need to shadow the stdlib "testing" package can alias:
//
//	import (
//	    "testing"
//	    registrytesting "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/registry/testing"
//	)
package registrytesting

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// Sentinel errors for InMemoryEntityTypeRegistry. Callers may compare using
// errors.Is to distinguish the specific failure kind from a generic error.
var (
	// ErrDuplicateEntityType is returned by Register when metadata.ID is
	// already present in the registry.
	ErrDuplicateEntityType = errors.New("entity type already registered")

	// ErrUnknownEntityType is returned by Update, Delete, and
	// SetOwningDetector when the given ID is not registered.
	ErrUnknownEntityType = errors.New("entity type not registered")

	// ErrOwnerConflict is returned by SetOwningDetector when the category
	// is already owned by a different detector.
	ErrOwnerConflict = errors.New("entity type already owned by a different detector")
)

// InMemoryEntityTypeRegistry is a thread-safe, in-memory implementation of
// pipelineport.EntityTypeRegistry. It is intended for use in tests only.
//
// Zero value is not usable; construct via NewInMemoryEntityTypeRegistry.
type InMemoryEntityTypeRegistry struct {
	mu    sync.RWMutex
	types map[pipeline.EntityType]pipeline.EntityTypeMetadata
}

// NewInMemoryEntityTypeRegistry creates a ready-to-use in-memory registry.
func NewInMemoryEntityTypeRegistry() *InMemoryEntityTypeRegistry {
	return &InMemoryEntityTypeRegistry{
		types: make(map[pipeline.EntityType]pipeline.EntityTypeMetadata),
	}
}

// Register adds a new entity type to the registry.
//
// Returns ErrDuplicateEntityType if metadata.ID is already registered.
func (r *InMemoryEntityTypeRegistry) Register(_ context.Context, metadata pipeline.EntityTypeMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[metadata.ID]; exists {
		return fmt.Errorf("Register %q: %w", metadata.ID, ErrDuplicateEntityType)
	}
	r.types[metadata.ID] = metadata
	return nil
}

// Update mutates an existing entity type in the registry.
//
// Returns ErrUnknownEntityType if metadata.ID is not registered.
func (r *InMemoryEntityTypeRegistry) Update(_ context.Context, metadata pipeline.EntityTypeMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[metadata.ID]; !exists {
		return fmt.Errorf("Update %q: %w", metadata.ID, ErrUnknownEntityType)
	}
	r.types[metadata.ID] = metadata
	return nil
}

// Delete removes an entity type from the registry.
//
// Returns ErrUnknownEntityType if id is not registered.
func (r *InMemoryEntityTypeRegistry) Delete(_ context.Context, id pipeline.EntityType) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[id]; !exists {
		return fmt.Errorf("Delete %q: %w", id, ErrUnknownEntityType)
	}
	delete(r.types, id)
	return nil
}

// Get retrieves an entity type by its ID.
//
// Returns (zero, false, nil) when id is not registered — this is a miss,
// not an error.
func (r *InMemoryEntityTypeRegistry) Get(_ context.Context, id pipeline.EntityType) (pipeline.EntityTypeMetadata, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, found := r.types[id]
	return m, found, nil
}

// List returns all registered entity types as a slice. Order is not guaranteed
// to be stable across calls.
//
// Returns an empty (non-nil) slice when the registry is empty.
func (r *InMemoryEntityTypeRegistry) List(_ context.Context) ([]pipeline.EntityTypeMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]pipeline.EntityTypeMetadata, 0, len(r.types))
	for _, m := range r.types {
		out = append(out, m)
	}
	return out, nil
}

// SetOwningDetector claims (or clears) ownership of a category for the given
// detector.
//
// Four-case contract:
//
//   - no prior owner → detectorID is set; returns nil.
//   - same owner → idempotent; returns nil.
//   - empty detectorID → clears ownership (idempotent if already unowned); returns nil.
//   - different owner → returns ErrOwnerConflict; record is unchanged.
//   - unknown ID → returns ErrUnknownEntityType.
func (r *InMemoryEntityTypeRegistry) SetOwningDetector(_ context.Context, id pipeline.EntityType, detectorID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.types[id]
	if !exists {
		return fmt.Errorf("SetOwningDetector %q: %w", id, ErrUnknownEntityType)
	}

	// Clearing ownership (empty detectorID) is always allowed.
	if detectorID == "" {
		m.OwningDetector = ""
		r.types[id] = m
		return nil
	}

	// Idempotent: same owner calling again.
	if m.OwningDetector == detectorID {
		return nil
	}

	// No prior owner: claim it.
	if m.OwningDetector == "" {
		m.OwningDetector = detectorID
		r.types[id] = m
		return nil
	}

	// Different owner: conflict.
	return fmt.Errorf("SetOwningDetector %q (detector %q conflicts with owner %q): %w",
		id, detectorID, m.OwningDetector, ErrOwnerConflict)
}

// Compile-time assertion: InMemoryEntityTypeRegistry must satisfy the port.
var _ pipelineport.EntityTypeRegistry = (*InMemoryEntityTypeRegistry)(nil)
