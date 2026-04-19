package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// mockStageRegistry is a mock implementation of StageRegistry
type mockStageRegistry struct {
	factories map[string]pipelineport.StageFactory
}

func newMockStageRegistry() *mockStageRegistry {
	return &mockStageRegistry{
		factories: make(map[string]pipelineport.StageFactory),
	}
}

func (r *mockStageRegistry) Register(factory pipelineport.StageFactory) error {
	r.factories[factory.StageID()] = factory
	return nil
}

func (r *mockStageRegistry) Get(stageID string) (pipelineport.StageFactory, bool) {
	factory, ok := r.factories[stageID]
	return factory, ok
}

func (r *mockStageRegistry) List() []pipeline.StageDescriptor {
	descriptors := make([]pipeline.StageDescriptor, 0, len(r.factories))
	for _, factory := range r.factories {
		descriptors = append(descriptors, factory.Descriptor())
	}
	return descriptors
}

func (r *mockStageRegistry) ListByType(stageType pipeline.StageType) []pipeline.StageDescriptor {
	descriptors := make([]pipeline.StageDescriptor, 0)
	for _, factory := range r.factories {
		desc := factory.Descriptor()
		if desc.Type == stageType {
			descriptors = append(descriptors, desc)
		}
	}
	return descriptors
}

// mockStage is a mock implementation of Stage
type mockStage struct {
	descriptor  pipeline.StageDescriptor
	processFunc func(ctx context.Context, input any) (any, error)
}

func (s *mockStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

func (s *mockStage) Process(ctx context.Context, input any) (any, error) {
	if s.processFunc != nil {
		return s.processFunc(ctx, input)
	}
	return input, nil
}

// mockStageFactory is a mock implementation of StageFactory
type mockStageFactory struct {
	descriptor pipeline.StageDescriptor
	createFunc func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error)
}

func (f *mockStageFactory) StageID() string {
	return f.descriptor.ID
}

func (f *mockStageFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

func (f *mockStageFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	if f.createFunc != nil {
		return f.createFunc(config, capabilities)
	}
	return &mockStage{descriptor: f.descriptor}, nil
}

func (f *mockStageFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

func TestPipelineBuilder_Build_SkipsOptionalStagesThatFailToCreate(t *testing.T) {
	registry := newMockStageRegistry()

	// Register a stage with only optional capabilities that will fail to create
	optionalFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "optional-stage",
			Name:        "Optional Stage",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeEmbeddedChunk,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
			},
		},
		createFunc: func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
			// Simulate failure (e.g., wrong capability type in set)
			return nil, errors.New("failed to create optional stage")
		},
	}

	// Register a required stage that will succeed
	requiredFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "required-stage",
			Name:        "Required Stage",
			Type:        pipeline.StageTypeLoader,
			InputShape:  pipeline.ShapeEmbeddedChunk,
			OutputShape: pipeline.ShapeIndexedDoc,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
			},
		},
		createFunc: func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
			return &mockStage{
				descriptor: pipeline.StageDescriptor{
					ID:          "required-stage",
					InputShape:  pipeline.ShapeEmbeddedChunk,
					OutputShape: pipeline.ShapeIndexedDoc,
				},
			}, nil
		},
	}

	_ = registry.Register(optionalFactory)
	_ = registry.Register(requiredFactory)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "optional-stage", Enabled: true},
			{StageID: "required-stage", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	execPipeline, err := builder.Build(pipelineDef, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if execPipeline == nil {
		t.Fatal("expected pipeline to be created")
	}

	// Verify only the required stage was added
	stages := execPipeline.Stages()
	if len(stages) != 1 {
		t.Errorf("expected 1 stage (required only), got %d", len(stages))
	}

	if stages[0].Descriptor().ID != "required-stage" {
		t.Errorf("expected required-stage, got %s", stages[0].Descriptor().ID)
	}
}

func TestPipelineBuilder_Build_FailsWhenRequiredStageFailsToCreate(t *testing.T) {
	registry := newMockStageRegistry()

	// Register a stage with required capabilities that will fail to create
	requiredFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "required-stage",
			Name:        "Required Stage",
			Type:        pipeline.StageTypeLoader,
			InputShape:  pipeline.ShapeEmbeddedChunk,
			OutputShape: pipeline.ShapeIndexedDoc,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
			},
		},
		createFunc: func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
			return nil, errors.New("required capability not available")
		},
	}

	_ = registry.Register(requiredFactory)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "required-stage", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	_, err := builder.Build(pipelineDef, capabilities)

	if err == nil {
		t.Error("expected error when required stage fails to create")
	}
}

func TestPipelineBuilder_Build_SkipsDisabledStages(t *testing.T) {
	registry := newMockStageRegistry()

	factory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "test-stage",
			Name:        "Test Stage",
			Type:        pipeline.StageTypeTransformer,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeChunk,
		},
	}

	_ = registry.Register(factory)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "test-stage", Enabled: false},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	_, err := builder.Build(pipelineDef, capabilities)

	// Should fail because there are no enabled stages
	if err == nil {
		t.Error("expected error when no enabled stages")
	}
}

func TestPipelineBuilder_Build_SuccessfulBuild(t *testing.T) {
	registry := newMockStageRegistry()

	factory1 := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "stage-1",
			Name:        "Stage 1",
			Type:        pipeline.StageTypeTransformer,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeChunk,
		},
	}

	factory2 := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "stage-2",
			Name:        "Stage 2",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeEmbeddedChunk,
		},
	}

	_ = registry.Register(factory1)
	_ = registry.Register(factory2)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "stage-1", Enabled: true},
			{StageID: "stage-2", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	execPipeline, err := builder.Build(pipelineDef, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if execPipeline == nil {
		t.Fatal("expected pipeline to be created")
	}

	stages := execPipeline.Stages()
	if len(stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(stages))
	}
}

func TestPipelineBuilder_Build_MixedOptionalAndRequiredStages(t *testing.T) {
	registry := newMockStageRegistry()

	// Optional stage that fails to create
	optionalFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "optional-stage",
			Name:        "Optional Stage",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeChunk, // Same output as input for pass-through
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
			},
		},
		createFunc: func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
			return nil, errors.New("optional capability not available")
		},
	}

	// Required stage that succeeds
	requiredFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "required-stage",
			Name:        "Required Stage",
			Type:        pipeline.StageTypeLoader,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeIndexedDoc,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
			},
		},
	}

	_ = registry.Register(optionalFactory)
	_ = registry.Register(requiredFactory)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "optional-stage", Enabled: true},
			{StageID: "required-stage", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	execPipeline, err := builder.Build(pipelineDef, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	stages := execPipeline.Stages()
	if len(stages) != 1 {
		t.Errorf("expected 1 stage (required only), got %d", len(stages))
	}

	if stages[0].Descriptor().ID != "required-stage" {
		t.Errorf("expected required-stage, got %s", stages[0].Descriptor().ID)
	}
}

func TestPipelineBuilder_Build_StageWithBothRequiredAndOptionalCapabilities(t *testing.T) {
	registry := newMockStageRegistry()

	// Stage with both required and optional capabilities
	mixedFactory := &mockStageFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          "mixed-stage",
			Name:        "Mixed Stage",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeEmbeddedChunk,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
			},
		},
		createFunc: func(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
			// Fail because optional is missing (even though required is present)
			return nil, errors.New("some capability issue")
		},
	}

	_ = registry.Register(mixedFactory)

	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "mixed-stage", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	// Should fail because stage has required capabilities
	_, err := builder.Build(pipelineDef, capabilities)

	if err == nil {
		t.Error("expected error when stage with required capabilities fails to create")
	}
}

func TestPipelineBuilder_Validate_EmptyPipeline(t *testing.T) {
	registry := newMockStageRegistry()
	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:     "empty-pipeline",
		Name:   "Empty Pipeline",
		Type:   pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{},
	}

	err := builder.Validate(pipelineDef)

	if err == nil {
		t.Error("expected error for empty pipeline")
	}
}

func TestPipelineBuilder_Validate_MissingPipelineID(t *testing.T) {
	registry := newMockStageRegistry()
	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "test-stage", Enabled: true},
		},
	}

	err := builder.Validate(pipelineDef)

	if err == nil {
		t.Error("expected error for missing pipeline ID")
	}
}

func TestPipelineBuilder_Build_UnknownStageID(t *testing.T) {
	registry := newMockStageRegistry()
	builder := NewPipelineBuilder(registry)

	pipelineDef := pipeline.PipelineDefinition{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "unknown-stage", Enabled: true},
		},
	}

	capabilities := pipeline.NewCapabilitySet()

	_, err := builder.Build(pipelineDef, capabilities)

	if err == nil {
		t.Error("expected error for unknown stage ID")
	}
}

func TestExecutablePipeline_Run(t *testing.T) {
	processedStages := []string{}

	stage1 := &mockStage{
		descriptor: pipeline.StageDescriptor{ID: "stage-1"},
		processFunc: func(ctx context.Context, input any) (any, error) {
			processedStages = append(processedStages, "stage-1")
			return "output-1", nil
		},
	}

	stage2 := &mockStage{
		descriptor: pipeline.StageDescriptor{ID: "stage-2"},
		processFunc: func(ctx context.Context, input any) (any, error) {
			processedStages = append(processedStages, "stage-2")
			if input != "output-1" {
				t.Errorf("expected input 'output-1', got %v", input)
			}
			return "output-2", nil
		},
	}

	execPipeline := &executablePipeline{
		definition: pipeline.PipelineDefinition{
			ID:   "test-pipeline",
			Name: "Test Pipeline",
		},
		stages: []pipelineport.Stage{stage1, stage2},
	}

	result, err := execPipeline.Run(context.Background(), "input")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "output-2" {
		t.Errorf("expected result 'output-2', got %v", result)
	}

	if len(processedStages) != 2 {
		t.Errorf("expected 2 stages to be processed, got %d", len(processedStages))
	}
}

func TestExecutablePipeline_Run_StageError(t *testing.T) {
	stage1 := &mockStage{
		descriptor: pipeline.StageDescriptor{ID: "stage-1"},
		processFunc: func(ctx context.Context, input any) (any, error) {
			return "output-1", nil
		},
	}

	stage2 := &mockStage{
		descriptor: pipeline.StageDescriptor{ID: "stage-2"},
		processFunc: func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("stage processing error")
		},
	}

	execPipeline := &executablePipeline{
		definition: pipeline.PipelineDefinition{
			ID:   "test-pipeline",
			Name: "Test Pipeline",
		},
		stages: []pipelineport.Stage{stage1, stage2},
	}

	_, err := execPipeline.Run(context.Background(), "input")

	if err == nil {
		t.Error("expected error from stage processing")
	}
}
