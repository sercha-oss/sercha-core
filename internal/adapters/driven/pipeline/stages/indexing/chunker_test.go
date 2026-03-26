package indexing

import (
	"context"
	"strings"
	"testing"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
)

func TestChunkerFactory_Create(t *testing.T) {
	factory := NewChunkerFactory()

	if factory.StageID() != ChunkerStageID {
		t.Errorf("expected stage ID %s, got %s", ChunkerStageID, factory.StageID())
	}

	desc := factory.Descriptor()
	if desc.Type != pipeline.StageTypeTransformer {
		t.Errorf("expected type Transformer, got %s", desc.Type)
	}
	if desc.InputShape != pipeline.ShapeContent {
		t.Errorf("expected input shape Content, got %s", desc.InputShape)
	}
	if desc.OutputShape != pipeline.ShapeChunk {
		t.Errorf("expected output shape Chunk, got %s", desc.OutputShape)
	}

	// Create with default config
	config := pipeline.StageConfig{StageID: ChunkerStageID, Enabled: true}
	stage, err := factory.Create(config, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if stage == nil {
		t.Error("expected stage to be created")
	}
}

func TestChunkerStage_Process(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(20)},
	}
	stage, _ := factory.Create(config, nil)

	tests := []struct {
		name        string
		input       *pipeline.IndexingInput
		minChunks   int
		expectError bool
	}{
		{
			name: "short content single chunk",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-1",
				Content:    "This is a short piece of content.",
			},
			minChunks: 1,
		},
		{
			name: "long content multiple chunks",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-2",
				Content:    strings.Repeat("This is a test sentence that will be chunked. ", 20),
			},
			minChunks: 2,
		},
		{
			name: "empty content",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-3",
				Content:    "",
			},
			minChunks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stage.Process(context.Background(), tt.input)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil {
				chunks, ok := result.([]*pipeline.Chunk)
				if !ok {
					t.Fatal("expected result to be []*pipeline.Chunk")
				}
				if len(chunks) < tt.minChunks {
					t.Errorf("expected at least %d chunks, got %d", tt.minChunks, len(chunks))
				}

				// Verify chunk properties
				for i, chunk := range chunks {
					if chunk.DocumentID != tt.input.DocumentID {
						t.Errorf("chunk %d: expected document ID %s, got %s", i, tt.input.DocumentID, chunk.DocumentID)
					}
					if chunk.Position != i {
						t.Errorf("chunk %d: expected position %d, got %d", i, i, chunk.Position)
					}
					if chunk.ID == "" {
						t.Errorf("chunk %d: expected non-empty ID", i)
					}
				}
			}
		})
	}
}

func TestChunkerStage_InvalidInput(t *testing.T) {
	factory := NewChunkerFactory()
	stage, _ := factory.Create(pipeline.StageConfig{}, nil)

	// Test with invalid input type
	_, err := stage.Process(context.Background(), "invalid input")
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}
