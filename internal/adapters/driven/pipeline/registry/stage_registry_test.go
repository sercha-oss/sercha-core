package registry

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// mockStageFactory is a test double for StageFactory
type mockStageFactory struct {
	id         string
	descriptor pipeline.StageDescriptor
}

func newMockStageFactory(id string, stageType pipeline.StageType) *mockStageFactory {
	return &mockStageFactory{
		id: id,
		descriptor: pipeline.StageDescriptor{
			ID:          id,
			Name:        "Mock " + id,
			Type:        stageType,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeChunk,
			Cardinality: pipeline.CardinalityOneToMany,
			Version:     "1.0.0",
		},
	}
}

func (f *mockStageFactory) StageID() string                            { return f.id }
func (f *mockStageFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *mockStageFactory) Validate(config pipeline.StageConfig) error { return nil }
func (f *mockStageFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	return &mockStage{descriptor: f.descriptor}, nil
}

type mockStage struct {
	descriptor pipeline.StageDescriptor
}

func (s *mockStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }
func (s *mockStage) Process(ctx context.Context, input any) (any, error) {
	return input, nil
}

func TestStageRegistry_Register(t *testing.T) {
	registry := NewStageRegistry()

	factory := newMockStageFactory("test-chunker", pipeline.StageTypeTransformer)
	err := registry.Register(factory)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Duplicate registration should fail
	err = registry.Register(factory)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestStageRegistry_Get(t *testing.T) {
	registry := NewStageRegistry()
	factory := newMockStageFactory("test-chunker", pipeline.StageTypeTransformer)
	_ = registry.Register(factory)

	// Get existing factory
	got, ok := registry.Get("test-chunker")
	if !ok {
		t.Error("expected factory to be found")
	}
	if got.StageID() != "test-chunker" {
		t.Errorf("expected stage ID 'test-chunker', got %s", got.StageID())
	}

	// Get non-existing factory
	_, ok = registry.Get("non-existent")
	if ok {
		t.Error("expected factory to not be found")
	}
}

func TestStageRegistry_List(t *testing.T) {
	registry := NewStageRegistry()
	_ = registry.Register(newMockStageFactory("chunker", pipeline.StageTypeTransformer))
	_ = registry.Register(newMockStageFactory("embedder", pipeline.StageTypeEnricher))
	_ = registry.Register(newMockStageFactory("loader", pipeline.StageTypeLoader))

	descriptors := registry.List()
	if len(descriptors) != 3 {
		t.Errorf("expected 3 descriptors, got %d", len(descriptors))
	}
}

func TestStageRegistry_ListByType(t *testing.T) {
	registry := NewStageRegistry()
	_ = registry.Register(newMockStageFactory("chunker", pipeline.StageTypeTransformer))
	_ = registry.Register(newMockStageFactory("embedder", pipeline.StageTypeEnricher))
	_ = registry.Register(newMockStageFactory("embedder2", pipeline.StageTypeEnricher))

	enrichers := registry.ListByType(pipeline.StageTypeEnricher)
	if len(enrichers) != 2 {
		t.Errorf("expected 2 enrichers, got %d", len(enrichers))
	}

	transformers := registry.ListByType(pipeline.StageTypeTransformer)
	if len(transformers) != 1 {
		t.Errorf("expected 1 transformer, got %d", len(transformers))
	}
}
