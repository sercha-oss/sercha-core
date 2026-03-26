package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

// SearchExecutor executes search pipelines.
type SearchExecutor struct {
	builder          pipelineport.PipelineBuilder
	pipelineRegistry pipelineport.PipelineRegistry
	capRegistry      pipelineport.CapabilityRegistry
}

// NewSearchExecutor creates a new search executor.
func NewSearchExecutor(
	builder pipelineport.PipelineBuilder,
	pipelineRegistry pipelineport.PipelineRegistry,
	capRegistry pipelineport.CapabilityRegistry,
) *SearchExecutor {
	return &SearchExecutor{
		builder:          builder,
		pipelineRegistry: pipelineRegistry,
		capRegistry:      capRegistry,
	}
}

// Execute runs a search pipeline.
func (e *SearchExecutor) Execute(
	ctx context.Context,
	sctx *pipeline.SearchContext,
	input *pipeline.SearchInput,
) (*pipeline.SearchOutput, error) {
	startTime := time.Now()
	stageTimings := make(map[string]int64)

	// Get pipeline definition
	def, ok := e.pipelineRegistry.Get(sctx.PipelineID)
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", sctx.PipelineID)
	}

	// Collect required capabilities from all stages
	requiredCaps := e.collectRequiredCapabilities(def)

	// Build capability set
	capabilities, err := e.capRegistry.BuildCapabilitySet(requiredCaps)
	if err != nil {
		return nil, fmt.Errorf("failed to build capability set: %w", err)
	}
	sctx.Capabilities = capabilities

	// Build executable pipeline
	execPipeline, err := e.builder.Build(def, capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	// Run pipeline with timing
	result, err := e.runWithTiming(ctx, execPipeline, input, stageTimings)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Convert result to SearchOutput
	output, ok := result.(*pipeline.SearchOutput)
	if !ok {
		return nil, fmt.Errorf("unexpected pipeline output type: %T", result)
	}

	// Add timing information
	output.Timing = pipeline.ExecutionTiming{
		TotalMs: time.Since(startTime).Milliseconds(),
		StageMs: stageTimings,
	}

	return output, nil
}

// runWithTiming executes the pipeline while collecting per-stage timing.
func (e *SearchExecutor) runWithTiming(
	ctx context.Context,
	execPipeline pipelineport.ExecutablePipeline,
	input any,
	timings map[string]int64,
) (any, error) {
	current := input
	stages := execPipeline.Stages()

	for _, stage := range stages {
		stageStart := time.Now()
		desc := stage.Descriptor()

		output, err := stage.Process(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("stage %s failed: %w", desc.ID, err)
		}

		timings[desc.ID] = time.Since(stageStart).Milliseconds()
		current = output
	}

	return current, nil
}

// collectRequiredCapabilities collects all capability requirements from pipeline stages.
func (e *SearchExecutor) collectRequiredCapabilities(def pipeline.PipelineDefinition) []pipeline.CapabilityRequirement {
	// For now, return common search capabilities
	// In a full implementation, we'd iterate through stage descriptors
	return []pipeline.CapabilityRequirement{
		{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityOptional},
		{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
		{Type: pipeline.CapabilityLLM, Mode: pipeline.CapabilityOptional},
	}
}

// Ensure SearchExecutor implements the interface.
var _ pipelineport.SearchExecutor = (*SearchExecutor)(nil)
