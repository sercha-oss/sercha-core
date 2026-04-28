package pipeline

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// PipelineExecutor runs complete pipelines.
type PipelineExecutor interface {
	// Execute runs a pipeline with the given input.
	// Returns the final output from the last stage.
	Execute(ctx context.Context, def pipeline.PipelineDefinition, input any) (any, error)
}

// IndexingExecutor is a specialized executor for indexing pipelines.
type IndexingExecutor interface {
	// Execute runs an indexing pipeline.
	// Returns IndexingOutput with document and chunk IDs.
	Execute(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error)

	// ExecuteBatch runs an indexing pipeline for multiple documents.
	ExecuteBatch(ctx context.Context, pctx *pipeline.IndexingContext, inputs []*pipeline.IndexingInput) ([]*pipeline.IndexingOutput, error)
}

// SearchExecutor is a specialized executor for search pipelines.
type SearchExecutor interface {
	// Execute runs a search pipeline.
	// Returns SearchOutput with results and timing.
	Execute(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error)
}

// PipelineBuilder constructs executable pipelines from definitions.
type PipelineBuilder interface {
	// Build constructs a pipeline from a definition.
	// Validates stage compatibility and resolves capabilities.
	Build(def pipeline.PipelineDefinition, capabilities *pipeline.CapabilitySet) (ExecutablePipeline, error)

	// Validate checks if a pipeline definition is valid.
	// Verifies stage shapes are compatible and required capabilities exist.
	Validate(def pipeline.PipelineDefinition) error
}

// ExecutablePipeline is a compiled pipeline ready for execution.
type ExecutablePipeline interface {
	// Definition returns the pipeline definition.
	Definition() pipeline.PipelineDefinition

	// Stages returns the instantiated stages in order.
	Stages() []Stage

	// Run executes the pipeline with input.
	Run(ctx context.Context, input any) (any, error)
}

// PipelineSelector selects the appropriate pipeline for a request.
type PipelineSelector interface {
	// SelectIndexing selects the indexing pipeline to use.
	// Currently returns the default indexing pipeline.
	SelectIndexing(ctx context.Context) (pipeline.PipelineDefinition, error)

	// SelectSearch selects a search pipeline based on user preference and availability.
	// If pipelineID is empty, returns the highest-priority enabled pipeline.
	SelectSearch(ctx context.Context, pipelineID string) (pipeline.PipelineDefinition, error)

	// ListAvailableSearch returns all search pipelines available to the user.
	ListAvailableSearch(ctx context.Context) ([]pipeline.PipelineDefinition, error)
}
