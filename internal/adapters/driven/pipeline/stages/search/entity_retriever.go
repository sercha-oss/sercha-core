package search

import (
	"context"
	"log/slog"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/entitydetect"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const EntityRetrieverStageID = "entity-retriever"

// EntityRetrieverFactory creates the search-time entity-retriever stage.
//
// The retriever is a pure cache reader against entity_register. The cache is
// populated by the indexing-time entity-extractor — keeping the LLM call out
// of the search hot path, so search latency is bounded by an in-process
// PostgreSQL lookup rather than a per-candidate LLM round-trip.
//
// register, registry, and contextStore are application-internal ports passed
// via the constructor (not capabilities). All may be nil — the stage degrades
// to a pass-through. The contextStore is required for the cache key to align
// with the indexing-time extractor when an admin has set extra prompt
// context; if it's nil here but non-nil there, lookups will miss.
type EntityRetrieverFactory struct {
	descriptor   pipeline.StageDescriptor
	register     driven.EntityRegister
	registry     pipelineport.EntityTypeRegistry
	contextStore driven.ExtractionContextStore
}

// NewEntityRetrieverFactory creates a new entity-retriever factory.
func NewEntityRetrieverFactory(register driven.EntityRegister, registry pipelineport.EntityTypeRegistry, contextStore driven.ExtractionContextStore) *EntityRetrieverFactory {
	return &EntityRetrieverFactory{
		descriptor: pipeline.StageDescriptor{
			ID:           EntityRetrieverStageID,
			Name:         "Entity Retriever",
			Type:         pipeline.StageTypeEnricher,
			InputShape:   pipeline.ShapeCandidate,
			OutputShape:  pipeline.ShapeCandidate,
			Cardinality:  pipeline.CardinalityOneToOne,
			Capabilities: nil, // pure cache reader, no external services
			Version:      "1.0.0",
		},
		register:     register,
		registry:     registry,
		contextStore: contextStore,
	}
}

// StageID returns the stage identifier.
func (f *EntityRetrieverFactory) StageID() string { return f.descriptor.ID }

// Descriptor returns the stage descriptor.
func (f *EntityRetrieverFactory) Descriptor() pipeline.StageDescriptor { return f.descriptor }

// Validate validates the stage configuration. No config is required.
func (f *EntityRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

// Create creates a new entity-retriever stage.
func (f *EntityRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	return &EntityRetrieverStage{
		descriptor:   f.descriptor,
		register:     f.register,
		registry:     f.registry,
		contextStore: f.contextStore,
		logger:       slog.Default(),
	}, nil
}

// EntityRetrieverStage attaches cached entity spans to search candidates.
//
// The stage is read-only: it never calls the LLM, never writes to the cache.
// On a cache miss for a candidate the candidate passes through without
// entity metadata; downstream consumers interpret the absence of the
// "entities" key according to their own contract (typically: treat it as
// "no detected entities" and take the no-op path).
//
// Caller-source gate: detection lookups are skipped entirely when the caller
// source is not CallerSourceMCP. The metadata this stage attaches is only
// used by stages that themselves opt in to MCP-only execution; running
// cache reads for direct callers adds latency for metadata that would be
// discarded.
type EntityRetrieverStage struct {
	descriptor   pipeline.StageDescriptor
	register     driven.EntityRegister
	registry     pipelineport.EntityTypeRegistry
	contextStore driven.ExtractionContextStore
	logger       *slog.Logger
}

// Descriptor returns the stage descriptor.
func (s *EntityRetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

// Process attaches cached entity spans to each candidate.
func (s *EntityRetrieverStage) Process(ctx context.Context, input any) (any, error) {
	candidates, ok := input.([]*pipeline.Candidate)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Candidate"}
	}

	// Caller-source gate: only attach entities for MCP callers.
	if searchCtx, ok := pipeline.SearchContextFromContext(ctx); !ok || searchCtx == nil || searchCtx.Caller == nil || searchCtx.Caller.Source != domain.CallerSourceMCP {
		return candidates, nil
	}

	// No cache wired — pass through with no metadata.
	if s.register == nil {
		return candidates, nil
	}

	categories, ok := s.loadCategories(ctx)
	if !ok {
		// Registry unavailable or empty — nothing to look up. Candidates
		// pass through without entity metadata, the masker treats this
		// as "no detected entities", and the result is unmasked.
		return candidates, nil
	}
	// Load the extra prompt context so the cache key matches the indexing
	// extractor. Empty string is the default; on error degrade to empty —
	// which will produce a cache miss if the indexer hashed against a real
	// context, but a miss is harmless (candidate just passes through with
	// no entity metadata).
	extraContext := ""
	if s.contextStore != nil {
		if c, err := s.contextStore.Get(ctx); err != nil {
			s.logger.Warn("entity-retriever: extraction context fetch failed; proceeding without",
				"stage", s.descriptor.ID,
				"error", err,
			)
		} else {
			extraContext = c
		}
	}
	analyzerVersion := entitydetect.AnalyzerVersion(categories, extraContext)

	for _, c := range candidates {
		s.attachSpans(ctx, c, analyzerVersion)
	}
	return candidates, nil
}

// attachSpans looks up the cached spans for a single candidate and attaches
// them to its Metadata. On a cache miss or any error, the candidate is left
// unchanged (downstream stages must tolerate the absence of "entities").
func (s *EntityRetrieverStage) attachSpans(ctx context.Context, c *pipeline.Candidate, analyzerVersion string) {
	contentSHA := entitydetect.ContentSHA256(c.Content)
	analysis, hit, err := s.register.Get(ctx, c.DocumentID, contentSHA, analyzerVersion)
	if err != nil {
		s.logger.Warn("entity-retriever: register.Get failed; passing candidate through",
			"stage", s.descriptor.ID,
			"document_id", c.DocumentID,
			"error", err,
		)
		return
	}
	if !hit {
		// Cache miss is expected when:
		//  - Indexing happened before entity-extractor was wired in.
		//  - The taxonomy changed (analyzerVersion bumped) and re-indexing
		//    hasn't reached this document yet.
		//  - The chunker emitted a chunk whose content differs from the full
		//    document body the indexing extractor saw.
		// In all cases the candidate passes through without entity metadata.
		return
	}

	spans := analysis.Spans
	summary := make(map[pipeline.EntityType]int, len(spans))
	for _, sp := range spans {
		summary[sp.Type]++
	}

	if c.Metadata == nil {
		c.Metadata = make(map[string]any)
	}
	c.Metadata["entities"] = spans
	c.Metadata["entity_summary"] = summary
}

// loadCategories reads the active taxonomy from the registry. Returns
// (categories, true) when a non-empty list is available; (nil, false)
// otherwise. On (nil, false) the caller skips the cache lookup and lets
// candidates pass through without entity metadata — same as a cache miss.
//
// We do not fall back to a hard-coded list: the cache key on the writer
// side is keyed by the live registry contents, so any local fallback would
// hash to a different key and never hit the cache anyway.
func (s *EntityRetrieverStage) loadCategories(ctx context.Context) ([]pipeline.EntityTypeMetadata, bool) {
	if s.registry == nil {
		return nil, false
	}
	types, err := s.registry.List(ctx)
	if err != nil {
		s.logger.Warn("entity-retriever: registry.List failed; skipping cache lookup",
			"stage", s.descriptor.ID,
			"error", err,
		)
		return nil, false
	}
	if len(types) == 0 {
		return nil, false
	}
	return types, true
}

// Compile-time interface assertions.
var _ pipelineport.StageFactory = (*EntityRetrieverFactory)(nil)
var _ pipelineport.Stage = (*EntityRetrieverStage)(nil)
