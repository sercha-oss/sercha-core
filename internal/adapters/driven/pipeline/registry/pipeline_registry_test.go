package registry

import (
	"testing"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
)

func TestPipelineRegistry_Register(t *testing.T) {
	registry := NewPipelineRegistry()

	def := pipeline.PipelineDefinition{
		ID:   "default-indexing",
		Name: "Default Indexing Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "chunker", Enabled: true},
		},
	}

	err := registry.Register(def)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Duplicate registration should fail
	err = registry.Register(def)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestPipelineRegistry_Get(t *testing.T) {
	registry := NewPipelineRegistry()
	def := pipeline.PipelineDefinition{
		ID:   "default-indexing",
		Name: "Default Indexing Pipeline",
		Type: pipeline.PipelineTypeIndexing,
	}
	_ = registry.Register(def)

	// Get existing pipeline
	got, ok := registry.Get("default-indexing")
	if !ok {
		t.Error("expected pipeline to be found")
	}
	if got.ID != "default-indexing" {
		t.Errorf("expected ID 'default-indexing', got %s", got.ID)
	}

	// Get non-existing pipeline
	_, ok = registry.Get("non-existent")
	if ok {
		t.Error("expected pipeline to not be found")
	}
}

func TestPipelineRegistry_GetDefault(t *testing.T) {
	registry := NewPipelineRegistry()

	// Register indexing pipelines
	_ = registry.Register(pipeline.PipelineDefinition{
		ID:   "indexing-1",
		Type: pipeline.PipelineTypeIndexing,
	})
	_ = registry.Register(pipeline.PipelineDefinition{
		ID:   "indexing-2",
		Type: pipeline.PipelineTypeIndexing,
	})

	// First registered should be default
	def, ok := registry.GetDefault(pipeline.PipelineTypeIndexing)
	if !ok {
		t.Error("expected default to be found")
	}
	if def.ID != "indexing-1" {
		t.Errorf("expected default to be 'indexing-1', got %s", def.ID)
	}

	// Set new default
	err := registry.SetDefault(pipeline.PipelineTypeIndexing, "indexing-2")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	def, _ = registry.GetDefault(pipeline.PipelineTypeIndexing)
	if def.ID != "indexing-2" {
		t.Errorf("expected default to be 'indexing-2', got %s", def.ID)
	}
}

func TestPipelineRegistry_ListByType(t *testing.T) {
	registry := NewPipelineRegistry()

	_ = registry.Register(pipeline.PipelineDefinition{ID: "indexing-1", Type: pipeline.PipelineTypeIndexing})
	_ = registry.Register(pipeline.PipelineDefinition{ID: "indexing-2", Type: pipeline.PipelineTypeIndexing})
	_ = registry.Register(pipeline.PipelineDefinition{ID: "search-1", Type: pipeline.PipelineTypeSearch})

	indexing := registry.ListByType(pipeline.PipelineTypeIndexing)
	if len(indexing) != 2 {
		t.Errorf("expected 2 indexing pipelines, got %d", len(indexing))
	}

	search := registry.ListByType(pipeline.PipelineTypeSearch)
	if len(search) != 1 {
		t.Errorf("expected 1 search pipeline, got %d", len(search))
	}
}
