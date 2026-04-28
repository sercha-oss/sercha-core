package executor

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// IndexingExecutor executes indexing pipelines.
type IndexingExecutor struct {
	builder          pipelineport.PipelineBuilder
	pipelineRegistry pipelineport.PipelineRegistry
	capRegistry      pipelineport.CapabilityRegistry
	stageRegistry    pipelineport.StageRegistry
}

// NewIndexingExecutor creates a new indexing executor.
func NewIndexingExecutor(
	builder pipelineport.PipelineBuilder,
	pipelineRegistry pipelineport.PipelineRegistry,
	capRegistry pipelineport.CapabilityRegistry,
	stageRegistry pipelineport.StageRegistry,
) *IndexingExecutor {
	return &IndexingExecutor{
		builder:          builder,
		pipelineRegistry: pipelineRegistry,
		capRegistry:      capRegistry,
		stageRegistry:    stageRegistry,
	}
}

// Execute runs an indexing pipeline for a single document.
func (e *IndexingExecutor) Execute(
	ctx context.Context,
	pctx *pipeline.IndexingContext,
	input *pipeline.IndexingInput,
) (*pipeline.IndexingOutput, error) {
	// Get pipeline definition
	def, ok := e.pipelineRegistry.Get(pctx.PipelineID)
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", pctx.PipelineID)
	}

	// Apply preference-based stage filtering
	if pctx.Preferences != nil {
		def = e.applyPreferences(def, pctx.Preferences)
	}

	// Collect required capabilities from all stages
	requiredCaps := e.collectRequiredCapabilities(def)

	// Build capability set
	capabilities, err := e.capRegistry.BuildCapabilitySet(requiredCaps)
	if err != nil {
		return nil, fmt.Errorf("failed to build capability set: %w", err)
	}
	pctx.Capabilities = capabilities

	// Build executable pipeline
	execPipeline, err := e.builder.Build(def, capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	// Run pipeline
	result, err := execPipeline.Run(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Convert result to IndexingOutput
	switch v := result.(type) {
	case *pipeline.IndexingOutput:
		return v, nil
	case []*pipeline.Chunk:
		// When the last stage is a pass-through loader (e.g. bm25-loader in BM25-only mode),
		// the pipeline returns chunks. Build IndexingOutput from them.
		chunkIDs := make([]string, len(v))
		for i, c := range v {
			chunkIDs[i] = c.ID
		}
		docID := ""
		if len(v) > 0 {
			docID = v[0].DocumentID
		}
		return &pipeline.IndexingOutput{DocumentID: docID, ChunkIDs: chunkIDs}, nil
	default:
		return nil, fmt.Errorf("unexpected pipeline output type: %T", result)
	}
}

// ExecuteBatch runs an indexing pipeline for multiple documents.
func (e *IndexingExecutor) ExecuteBatch(
	ctx context.Context,
	pctx *pipeline.IndexingContext,
	inputs []*pipeline.IndexingInput,
) ([]*pipeline.IndexingOutput, error) {
	outputs := make([]*pipeline.IndexingOutput, 0, len(inputs))

	for _, input := range inputs {
		select {
		case <-ctx.Done():
			return outputs, ctx.Err()
		default:
		}

		output, err := e.Execute(ctx, pctx, input)
		if err != nil {
			// Log error but continue with other documents
			// In production, you might want configurable error handling
			continue
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}

// collectRequiredCapabilities collects all capability requirements from pipeline stages.
func (e *IndexingExecutor) collectRequiredCapabilities(def pipeline.PipelineDefinition) []pipeline.CapabilityRequirement {
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

// isStricterMode returns true if mode1 is stricter than mode2.
// Required > Optional > Fallback
func isStricterMode(mode1, mode2 pipeline.CapabilityMode) bool {
	priority := map[pipeline.CapabilityMode]int{
		pipeline.CapabilityRequired: 3,
		pipeline.CapabilityOptional: 2,
		pipeline.CapabilityFallback: 1,
	}
	return priority[mode1] > priority[mode2]
}

// applyPreferences filters pipeline stages based on user preferences.
func (e *IndexingExecutor) applyPreferences(def pipeline.PipelineDefinition, prefs *pipeline.StagePreferences) pipeline.PipelineDefinition {
	// Clone stages slice
	stages := make([]pipeline.StageConfig, len(def.Stages))
	copy(stages, def.Stages)

	for i := range stages {
		switch stages[i].StageID {
		case "doc-loader":
			if !prefs.TextIndexingEnabled {
				stages[i].Enabled = false
			}
		case "vector-loader", "embedder":
			if !prefs.EmbeddingIndexingEnabled {
				stages[i].Enabled = false
			}
		}
	}

	def.Stages = stages
	return def
}

// Ensure IndexingExecutor implements the interface.
var _ pipelineport.IndexingExecutor = (*IndexingExecutor)(nil)
