package normalisers

import (
	"sort"
	"strings"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.NormaliserRegistry = (*Registry)(nil)

// Registry implements NormaliserRegistry with priority-based selection.
// When multiple normalisers match a MIME type, the highest priority one is used.
type Registry struct {
	mu          sync.RWMutex
	normalisers []driven.Normaliser
}

// NewRegistry creates a new normaliser registry.
func NewRegistry() *Registry {
	return &Registry{
		normalisers: make([]driven.Normaliser, 0),
	}
}

// Register registers a normaliser.
// Normalisers are stored and later selected by priority.
func (r *Registry) Register(normaliser driven.Normaliser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.normalisers = append(r.normalisers, normaliser)
}

// Get retrieves the best-matching normaliser for a MIME type.
// Returns nil if no normaliser is registered for the type.
// When multiple match, the highest priority normaliser is returned.
func (r *Registry) Get(mimeType string) driven.Normaliser {
	matches := r.GetAll(mimeType)
	if len(matches) == 0 {
		return nil
	}
	return matches[0] // Already sorted by priority (highest first)
}

// GetAll retrieves all normalisers that match a MIME type, sorted by priority (highest first).
func (r *Registry) GetAll(mimeType string) []driven.Normaliser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []driven.Normaliser

	for _, n := range r.normalisers {
		if matchesMIMEType(n.SupportedTypes(), mimeType) {
			matches = append(matches, n)
		}
	}

	// Sort by priority (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Priority() > matches[j].Priority()
	})

	return matches
}

// List returns all registered MIME types.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typeSet := make(map[string]struct{})
	for _, n := range r.normalisers {
		for _, t := range n.SupportedTypes() {
			typeSet[t] = struct{}{}
		}
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// matchesMIMEType checks if any of the supported types match the given MIME type.
// Supports wildcard matching (e.g., "text/*" matches "text/plain").
func matchesMIMEType(supportedTypes []string, mimeType string) bool {
	// Normalize the input
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))

	// Strip charset and other parameters
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	for _, supported := range supportedTypes {
		supported = strings.ToLower(strings.TrimSpace(supported))

		// Exact match
		if supported == mimeType {
			return true
		}

		// Wildcard match (e.g., "text/*" matches "text/plain")
		if strings.HasSuffix(supported, "/*") {
			prefix := supported[:len(supported)-1] // "text/"
			if strings.HasPrefix(mimeType, prefix) {
				return true
			}
		}

		// Universal wildcard
		if supported == "*/*" {
			return true
		}
	}

	return false
}

// DefaultRegistry creates a registry with common normalisers pre-registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Register built-in normalisers in priority order
	r.Register(&PlaintextNormaliser{})
	r.Register(&MarkdownNormaliser{})
	r.Register(&HTMLNormaliser{})
	r.Register(&PDFNormaliser{})

	// Register connector-specific normalisers (high priority)
	r.Register(&GitHubIssueNormaliser{})
	r.Register(&GitHubPRNormaliser{})

	return r
}
