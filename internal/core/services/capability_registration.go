package services

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// RegisterBuiltinCapabilities populates the registry with the seven
// capabilities Core ships out of the box. Callers (Core's main, or any
// add-on that builds on Core) MUST call this before constructing the
// CapabilitiesService — the service iterates the registry and won't see
// capabilities registered later.
//
// Add-ons that need extra capabilities call registry.Register directly
// after this function returns. Registration order does not matter; the
// resolver walks dependencies generically.
//
// Returns an error if any descriptor fails to register (typically because
// it was already registered — duplicate registration is a programmer
// error, see the registry's Register contract).
func RegisterBuiltinCapabilities(registry domain.CapabilityRegistry) error {
	descriptors := []domain.CapabilityDescriptor{
		{
			Type:           domain.CapabilityTextIndexing,
			DisplayName:    "Text Indexing",
			Description:    "Indexes document text for keyword (BM25) search.",
			Phase:          domain.PipelinePhaseIndexing,
			BackendID:      "opensearch",
			Grants:         []domain.CapabilityType{domain.CapabilityBM25Search},
			DefaultEnabled: true,
		},
		{
			Type:           domain.CapabilityEmbeddingIndexing,
			DisplayName:    "Embedding Indexing",
			Description:    "Generates vector embeddings for semantic search.",
			Phase:          domain.PipelinePhaseIndexing,
			BackendID:      "pgvector",
			Grants:         []domain.CapabilityType{domain.CapabilityVectorSearch},
			DefaultEnabled: false,
		},
		{
			Type:           domain.CapabilityBM25Search,
			DisplayName:    "BM25 Search",
			Description:    "Keyword-based text search using BM25 ranking.",
			Phase:          domain.PipelinePhaseSearch,
			BackendID:      "opensearch",
			DependsOn:      []domain.CapabilityType{domain.CapabilityTextIndexing},
			DefaultEnabled: true,
		},
		{
			Type:           domain.CapabilityVectorSearch,
			DisplayName:    "Vector Search",
			Description:    "Semantic similarity search using vector embeddings.",
			Phase:          domain.PipelinePhaseSearch,
			BackendID:      "pgvector",
			DependsOn:      []domain.CapabilityType{domain.CapabilityEmbeddingIndexing},
			DefaultEnabled: true,
		},
		{
			Type:           domain.CapabilityQueryExpansion,
			DisplayName:    "Query Expansion",
			Description:    "Expands queries with related terms via the configured LLM.",
			Phase:          domain.PipelinePhaseSearch,
			BackendID:      "llm",
			DefaultEnabled: true,
		},
		{
			Type:           domain.CapabilityQueryRewriting,
			DisplayName:    "Query Rewriting",
			Description:    "Reformulates queries for better matching via the configured LLM.",
			Phase:          domain.PipelinePhaseSearch,
			BackendID:      "llm",
			DefaultEnabled: true,
		},
		{
			Type:           domain.CapabilitySummarization,
			DisplayName:    "Summarization",
			Description:    "Generates result snippets via the configured LLM.",
			Phase:          domain.PipelinePhaseSearch,
			BackendID:      "llm",
			DefaultEnabled: true,
		},
	}
	for _, d := range descriptors {
		if err := registry.Register(d); err != nil {
			return fmt.Errorf("register builtin %q: %w", d.Type, err)
		}
	}
	return nil
}

// NewBuiltinAvailabilityResolver builds an AvailabilityResolver that
// answers for the seven Core built-in capabilities, using the supplied
// ConfigProvider as the backend-status source.
//
// Returns false for any capability type it doesn't know about; compose
// with add-on resolvers via domain.NewCompositeAvailabilityResolver to
// extend coverage.
func NewBuiltinAvailabilityResolver(cfg driven.ConfigProvider) domain.AvailabilityResolver {
	return domain.AvailabilityFunc(func(ctx context.Context, t domain.CapabilityType) bool {
		caps := cfg.GetCapabilities()
		hasEmbeddings := len(caps.EmbeddingProviders) > 0
		hasLLM := len(caps.LLMProviders) > 0
		switch t {
		case domain.CapabilityTextIndexing:
			return caps.SearchEngineAvailable
		case domain.CapabilityEmbeddingIndexing:
			return hasEmbeddings && caps.VectorStoreAvailable
		case domain.CapabilityBM25Search:
			return caps.SearchEngineAvailable
		case domain.CapabilityVectorSearch:
			return hasEmbeddings && caps.VectorStoreAvailable
		case domain.CapabilityQueryExpansion:
			return hasLLM
		case domain.CapabilityQueryRewriting:
			return false // not yet implemented in Core
		case domain.CapabilitySummarization:
			return false // not yet implemented in Core
		}
		return false
	})
}
