package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

// IndexingExecutor executes indexing pipelines.
type IndexingExecutor struct {
	builder          pipelineport.PipelineBuilder
	pipelineRegistry pipelineport.PipelineRegistry
	capRegistry      pipelineport.CapabilityRegistry
	manifestStore    pipelineport.ManifestStore
}

// NewIndexingExecutor creates a new indexing executor.
func NewIndexingExecutor(
	builder pipelineport.PipelineBuilder,
	pipelineRegistry pipelineport.PipelineRegistry,
	capRegistry pipelineport.CapabilityRegistry,
	manifestStore pipelineport.ManifestStore,
) *IndexingExecutor {
	return &IndexingExecutor{
		builder:          builder,
		pipelineRegistry: pipelineRegistry,
		capRegistry:      capRegistry,
		manifestStore:    manifestStore,
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
	output, ok := result.(*pipeline.IndexingOutput)
	if !ok {
		return nil, fmt.Errorf("unexpected pipeline output type: %T", result)
	}

	return output, nil
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

	// Update manifest after batch completes
	if len(outputs) > 0 && e.manifestStore != nil {
		manifest := e.buildManifest(pctx, outputs)
		if err := e.manifestStore.Save(manifest); err != nil {
			// Log but don't fail - documents were indexed successfully
			_ = err
		}
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

		// Get stage descriptor to find capability requirements
		// This requires the stage registry, which we could inject
		// For now, we'll rely on the builder to handle this
	}

	result := make([]pipeline.CapabilityRequirement, 0, len(seen))
	for _, req := range seen {
		result = append(result, req)
	}
	return result
}

// buildManifest creates a produces manifest from indexing outputs.
func (e *IndexingExecutor) buildManifest(
	pctx *pipeline.IndexingContext,
	outputs []*pipeline.IndexingOutput,
) *pipeline.ProducesManifest {
	var docCount, chunkCount int64
	for _, out := range outputs {
		docCount++
		chunkCount += int64(len(out.ChunkIDs))
	}

	// Get capabilities that were used
	var producedCaps []pipeline.ProducedCapability
	if pctx.Capabilities != nil {
		for _, capType := range pctx.Capabilities.Types() {
			inst, ok := pctx.Capabilities.Get(capType)
			if ok {
				producedCaps = append(producedCaps, pipeline.ProducedCapability{
					Type:  capType,
					Store: inst.ID,
				})
			}
		}
	}

	return &pipeline.ProducesManifest{
		PipelineID:    pctx.PipelineID,
		ConnectorID:   pctx.ConnectorID,
		Timestamp:     time.Now(),
		Capabilities:  producedCaps,
		DocumentCount: docCount,
		ChunkCount:    chunkCount,
	}
}

// Ensure IndexingExecutor implements the interface.
var _ pipelineport.IndexingExecutor = (*IndexingExecutor)(nil)
