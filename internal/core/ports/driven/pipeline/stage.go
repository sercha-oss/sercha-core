package pipeline

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
)

// Stage is the core processing unit in a pipeline.
// Each stage transforms input data to output data according to its descriptor.
type Stage interface {
	// Descriptor returns the static metadata about this stage.
	Descriptor() pipeline.StageDescriptor

	// Process executes the stage logic on the input.
	// Input and output types depend on the stage's input/output shapes.
	// For 1:1 cardinality, input is a single item and output is a single item.
	// For 1:N cardinality, input is a single item and output is a slice.
	// For N:N cardinality, input is a slice and output is a slice.
	// For N:1 cardinality, input is a slice and output is a single item.
	Process(ctx context.Context, input any) (output any, err error)
}

// StageFactory creates configured stage instances.
// Each stage implementation registers a factory.
type StageFactory interface {
	// StageID returns the unique identifier for stages this factory creates.
	StageID() string

	// Descriptor returns the stage descriptor without creating an instance.
	Descriptor() pipeline.StageDescriptor

	// Create creates a new stage instance with the given configuration.
	// The capabilities set provides access to required dependencies.
	Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (Stage, error)

	// Validate checks if the configuration is valid for this stage.
	Validate(config pipeline.StageConfig) error
}

// IndexingStage is a specialized stage for indexing pipelines.
// It provides type-safe processing for indexing data.
type IndexingStage interface {
	Stage

	// ProcessIndexing is a type-safe version of Process for indexing.
	// Input shape determines the concrete type:
	// - ShapeContent: *pipeline.IndexingInput
	// - ShapeChunk: []*pipeline.Chunk
	// - ShapeEmbeddedChunk: []*pipeline.Chunk (with embeddings)
	ProcessIndexing(ctx context.Context, pctx *pipeline.IndexingContext, input any) (any, error)
}

// SearchStage is a specialized stage for search pipelines.
// It provides type-safe processing for search data.
type SearchStage interface {
	Stage

	// ProcessSearch is a type-safe version of Process for search.
	// Input shape determines the concrete type:
	// - ShapeQuery: *pipeline.SearchInput
	// - ShapeParsedQuery: *pipeline.ParsedQuery
	// - ShapeCandidate: []*pipeline.Candidate
	// - ShapeRankedResult: []*pipeline.Candidate (ranked)
	ProcessSearch(ctx context.Context, sctx *pipeline.SearchContext, input any) (any, error)
}
