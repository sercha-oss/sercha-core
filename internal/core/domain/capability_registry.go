package domain

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// CapabilityRegistry is a runtime registry of capability descriptors. The
// capabilities service iterates the registry to know which capabilities
// exist; consumers (built-in or add-on) register their descriptors at
// startup before the service handles requests.
//
// Registration order is not significant — descriptors expose their own
// dependency graph via DependsOn / Grants and the resolver walks it
// generically. Adding a new capability requires only registering a
// descriptor; no service-side code changes.
//
// Implementations must be safe for concurrent reads. Registration is
// expected to happen during startup (single-goroutine) and is not
// performance-critical.
type CapabilityRegistry interface {
	// Register adds a descriptor to the registry. Returns an error when a
	// descriptor with the same Type is already registered — duplicate
	// registration is a programmer error and must surface, not silently
	// overwrite.
	Register(d CapabilityDescriptor) error

	// All returns every registered descriptor. The returned slice is a
	// copy; mutation does not affect the registry.
	All() []CapabilityDescriptor

	// Get returns the descriptor for the given type, or false if no
	// descriptor with that type is registered.
	Get(t CapabilityType) (CapabilityDescriptor, bool)
}

// AvailabilityResolver answers whether a capability is currently available
// in the running system. Composable: a base resolver covers built-in
// backend checks (search engine, vector store, LLM) and consumers can wrap
// it to add new rules for their own capability types.
//
// Resolvers must be safe for concurrent use. They are called from request
// handling paths and should not perform blocking I/O — runtime services
// usually expose simple availability getters that the resolver consults.
type AvailabilityResolver interface {
	// IsAvailable returns true when the runtime can support this
	// capability now. Returning false for a capability that has been
	// registered is normal — it just means the backend isn't configured
	// or healthy at this moment.
	//
	// For an unknown capability type (one not registered) the resolver
	// SHOULD return false to surface the configuration mistake.
	IsAvailable(ctx context.Context, t CapabilityType) bool
}

// AvailabilityFunc is a function adapter for AvailabilityResolver.
type AvailabilityFunc func(ctx context.Context, t CapabilityType) bool

// IsAvailable implements AvailabilityResolver.
func (f AvailabilityFunc) IsAvailable(ctx context.Context, t CapabilityType) bool {
	return f(ctx, t)
}

// CompositeAvailabilityResolver chains multiple resolvers; the first one to
// return a definitive answer for a known capability wins. This lets Core
// ship a base resolver and add-ons compose their own rules without
// modifying Core's resolver.
//
// "Definitive answer" semantics: each resolver in the chain reports both a
// known/unknown signal and an availability bit. Use the
// AnswerableResolver wrapper or implement IsKnown directly when you need
// chain-aware resolution.
//
// For the common case of "either resolver says yes → yes", use
// resolverAny instead.
type CompositeAvailabilityResolver struct {
	resolvers []AvailabilityResolver
}

// NewCompositeAvailabilityResolver builds a chain. The first resolver that
// returns true for a given type wins; if none return true the answer is
// false.
//
// This means the chain is "OR" semantics: the capability is available
// if ANY resolver says it is. Suitable for the common pattern of a base
// resolver + add-on resolvers each contributing knowledge of their own
// capability set.
func NewCompositeAvailabilityResolver(resolvers ...AvailabilityResolver) *CompositeAvailabilityResolver {
	return &CompositeAvailabilityResolver{resolvers: resolvers}
}

// IsAvailable implements AvailabilityResolver.
func (c *CompositeAvailabilityResolver) IsAvailable(ctx context.Context, t CapabilityType) bool {
	for _, r := range c.resolvers {
		if r.IsAvailable(ctx, t) {
			return true
		}
	}
	return false
}

// inMemoryRegistry is the default CapabilityRegistry implementation. Lives
// in domain because the registry is conceptually domain — there is no
// adapter-side concern, just a thread-safe map.
type inMemoryRegistry struct {
	mu          sync.RWMutex
	descriptors map[CapabilityType]CapabilityDescriptor
}

// NewCapabilityRegistry creates an empty in-memory registry.
func NewCapabilityRegistry() CapabilityRegistry {
	return &inMemoryRegistry{descriptors: make(map[CapabilityType]CapabilityDescriptor)}
}

// Register implements CapabilityRegistry.
func (r *inMemoryRegistry) Register(d CapabilityDescriptor) error {
	if d.Type == "" {
		return fmt.Errorf("capability registry: descriptor has empty Type")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[d.Type]; exists {
		return fmt.Errorf("capability registry: %q already registered", d.Type)
	}
	r.descriptors[d.Type] = d
	return nil
}

// All implements CapabilityRegistry. Results are sorted by Type for
// deterministic output (UI rendering, tests, audit logs).
func (r *inMemoryRegistry) All() []CapabilityDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CapabilityDescriptor, 0, len(r.descriptors))
	for _, d := range r.descriptors {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// Get implements CapabilityRegistry.
func (r *inMemoryRegistry) Get(t CapabilityType) (CapabilityDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.descriptors[t]
	return d, ok
}
