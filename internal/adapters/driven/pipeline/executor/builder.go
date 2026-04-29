package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// PipelineBuilder constructs executable pipelines from definitions.
type PipelineBuilder struct {
	stageRegistry pipelineport.StageRegistry
	logger        *slog.Logger
}

// NewPipelineBuilder creates a new pipeline builder.
func NewPipelineBuilder(stageRegistry pipelineport.StageRegistry) *PipelineBuilder {
	return &PipelineBuilder{
		stageRegistry: stageRegistry,
		logger:        slog.Default(),
	}
}

// WithLogger returns a copy of the builder with the given logger. Used for
// per-stage timing instrumentation in built pipelines.
func (b *PipelineBuilder) WithLogger(logger *slog.Logger) *PipelineBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	return &PipelineBuilder{
		stageRegistry: b.stageRegistry,
		logger:        logger,
	}
}

// Build constructs a pipeline from a definition.
func (b *PipelineBuilder) Build(def pipeline.PipelineDefinition, capabilities *pipeline.CapabilitySet) (pipelineport.ExecutablePipeline, error) {
	if err := b.Validate(def); err != nil {
		return nil, fmt.Errorf("invalid pipeline definition: %w", err)
	}

	stages := make([]pipelineport.Stage, 0, len(def.Stages))

	for i, stageConfig := range def.Stages {
		if !stageConfig.Enabled {
			continue
		}

		factory, ok := b.stageRegistry.Get(stageConfig.StageID)
		if !ok {
			return nil, fmt.Errorf("stage factory not found: %s", stageConfig.StageID)
		}

		stage, err := factory.Create(stageConfig, capabilities)
		if err != nil {
			// Check if this stage has only optional capabilities
			descriptor := factory.Descriptor()
			requiredCaps := descriptor.GetRequiredCapabilities()

			// If stage has no required capabilities (all optional), skip it gracefully
			if len(requiredCaps) == 0 {
				// Log and skip this stage
				continue
			}

			// Stage has required capabilities, so fail the build
			return nil, fmt.Errorf("failed to create stage %d (%s): %w", i, stageConfig.StageID, err)
		}

		stages = append(stages, stage)
	}

	if len(stages) == 0 {
		return nil, fmt.Errorf("pipeline has no enabled stages")
	}

	return &executablePipeline{
		definition: def,
		stages:     stages,
		logger:     b.logger,
	}, nil
}

// Validate checks if a pipeline definition is valid.
func (b *PipelineBuilder) Validate(def pipeline.PipelineDefinition) error {
	if def.ID == "" {
		return fmt.Errorf("pipeline ID is required")
	}

	if len(def.Stages) == 0 {
		return fmt.Errorf("pipeline must have at least one stage")
	}

	var enabledStages []pipeline.StageDescriptor

	for i, stageConfig := range def.Stages {
		if !stageConfig.Enabled {
			continue
		}

		factory, ok := b.stageRegistry.Get(stageConfig.StageID)
		if !ok {
			return fmt.Errorf("stage %d: factory not found: %s", i, stageConfig.StageID)
		}

		if err := factory.Validate(stageConfig); err != nil {
			return fmt.Errorf("stage %d (%s): %w", i, stageConfig.StageID, err)
		}

		enabledStages = append(enabledStages, factory.Descriptor())
	}

	// Validate shape compatibility between adjacent stages
	for i := 1; i < len(enabledStages); i++ {
		prev := enabledStages[i-1]
		curr := enabledStages[i]

		if prev.OutputShape != curr.InputShape {
			return fmt.Errorf("shape mismatch between stages %d and %d: %s -> %s",
				i-1, i, prev.OutputShape, curr.InputShape)
		}
	}

	return nil
}

// executablePipeline is a compiled pipeline ready for execution.
type executablePipeline struct {
	definition pipeline.PipelineDefinition
	stages     []pipelineport.Stage
	logger     *slog.Logger
}

// Definition returns the pipeline definition.
func (p *executablePipeline) Definition() pipeline.PipelineDefinition {
	return p.definition
}

// Stages returns the instantiated stages in order.
func (p *executablePipeline) Stages() []pipelineport.Stage {
	return p.stages
}

// Run executes the pipeline with input.
func (p *executablePipeline) Run(ctx context.Context, input any) (any, error) {
	current := input
	logger := p.logger
	if logger == nil {
		logger = slog.Default()
	}

	for i, stage := range p.stages {
		desc := stage.Descriptor()
		start := time.Now()
		output, err := stage.Process(ctx, current)
		duration := time.Since(start)
		if err != nil {
			logger.Warn("pipeline stage failed",
				"phase", "stage",
				"pipeline_id", p.definition.ID,
				"stage_id", desc.ID,
				"stage_index", i,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
			return nil, fmt.Errorf("stage %d (%s) failed: %w", i, desc.ID, err)
		}
		logger.Debug("pipeline stage completed",
			"phase", "stage",
			"pipeline_id", p.definition.ID,
			"stage_id", desc.ID,
			"stage_index", i,
			"duration_ms", duration.Milliseconds(),
		)
		current = output
	}

	return current, nil
}

// Ensure types implement interfaces.
var (
	_ pipelineport.PipelineBuilder    = (*PipelineBuilder)(nil)
	_ pipelineport.ExecutablePipeline = (*executablePipeline)(nil)
)
