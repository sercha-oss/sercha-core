package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// SearchExecutor executes search pipelines.
type SearchExecutor struct {
	builder          pipelineport.PipelineBuilder
	pipelineRegistry pipelineport.PipelineRegistry
	capRegistry      pipelineport.CapabilityRegistry
	stageRegistry    pipelineport.StageRegistry
}

// NewSearchExecutor creates a new search executor.
func NewSearchExecutor(
	builder pipelineport.PipelineBuilder,
	pipelineRegistry pipelineport.PipelineRegistry,
	capRegistry pipelineport.CapabilityRegistry,
	stageRegistry pipelineport.StageRegistry,
) *SearchExecutor {
	return &SearchExecutor{
		builder:          builder,
		pipelineRegistry: pipelineRegistry,
		capRegistry:      capRegistry,
		stageRegistry:    stageRegistry,
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

	// Apply preference-based stage configuration
	if sctx.Preferences != nil {
		def = applyPreferences(def, sctx.Preferences)
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

	// Inject the SearchContext into the context so that stages can read caller
	// metadata (e.g. CallerSource for MCP-gated stages) without requiring a
	// separate parameter on the Stage interface.
	ctx = pipeline.SearchContextWithContext(ctx, sctx)

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
	seen := make(map[pipeline.CapabilityType]pipeline.CapabilityRequirement)

	for _, stageConfig := range def.Stages {
		if !stageConfig.Enabled {
			continue
		}

		// Look up factory via stage registry to get the descriptor
		factory, ok := e.stageRegistry.Get(stageConfig.StageID)
		if !ok {
			continue
		}

		desc := factory.Descriptor()
		for _, req := range desc.Capabilities {
			// Deduplicate by capability type, keeping the strictest mode
			// Required beats Optional beats Fallback
			existing, exists := seen[req.Type]
			if !exists || isStricterMode(req.Mode, existing.Mode) {
				seen[req.Type] = req
			}
		}
	}

	result := make([]pipeline.CapabilityRequirement, 0, len(seen))
	for _, req := range seen {
		result = append(result, req)
	}
	return result
}

// applyPreferences applies search-side admin preferences to the pipeline definition.
// Today this only honours vector_search, by setting the multi-retriever's
// disable_vector parameter so its runSearch skips the pgvector path. Other prefs
// (BM25-only, query expansion, etc.) can be plumbed here as additional cases.
func applyPreferences(def pipeline.PipelineDefinition, prefs *pipeline.StagePreferences) pipeline.PipelineDefinition {
	stages := make([]pipeline.StageConfig, len(def.Stages))
	copy(stages, def.Stages)

	for i := range stages {
		// multi-retriever has a special two-toggle treatment: vector_search
		// off doesn't disable the stage entirely (BM25 still needs to run);
		// it sets the disable_vector parameter instead.
		if stages[i].StageID == "multi-retriever" {
			if !prefs.IsEnabled(domain.CapabilityVectorSearch, true) {
				params := make(map[string]any, len(stages[i].Parameters)+1)
				for k, v := range stages[i].Parameters {
					params[k] = v
				}
				params["disable_vector"] = true
				stages[i].Parameters = params
			}
		}

		// Generic stage-level capability gate: any stage whose pipeline
		// definition sets parameters["toggle_capability"]: "<cap_id>" is
		// disabled when that capability is toggled off. Add-on stages
		// (e.g. masker gated on "masking", entity-retriever gated on
		// "entity_extraction") plug in via this mechanism without
		// requiring Core to know their capability ids.
		if cap, ok := stages[i].Parameters["toggle_capability"].(string); ok && cap != "" {
			if !prefs.IsEnabled(domain.CapabilityType(cap), true) {
				stages[i].Enabled = false
			}
		}
	}

	def.Stages = stages
	return def
}

// Ensure SearchExecutor implements the interface.
var _ pipelineport.SearchExecutor = (*SearchExecutor)(nil)
