package indexing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
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

func TestChunkerStage_FiltersNonTextChunks(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(10)},
	}
	stage, _ := factory.Create(config, nil)

	// Simulate a document that starts with valid markdown then has a base64 blob.
	// With chunk_size=100, the text portion and base64 portion will be in separate chunks.
	textPart := strings.Repeat("This is valid markdown content with spaces and normal text. ", 3)
	base64Part := strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 3)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-mixed",
		SourceID:   "src-1",
		Content:    textPart + base64Part,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	chunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatal("expected []*pipeline.Chunk")
	}

	// All returned chunks should be text content, not base64
	for i, chunk := range chunks {
		if isLikelyNonText(chunk.Content) {
			t.Errorf("chunk %d should have been filtered (non-text content): %q", i, chunk.Content[:min(60, len(chunk.Content))])
		}
	}

	// We should have fewer chunks than if no filtering occurred
	if len(chunks) == 0 {
		t.Error("expected at least one text chunk to survive filtering")
	}
}

func TestChunkerStage_AllNonTextContent(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(10)},
	}
	stage, _ := factory.Create(config, nil)

	// Document where ALL content is base64 — every chunk window will be non-text.
	// Previously this would panic with index out of bounds on chunks[len(chunks)-1]
	// because no chunks were appended but the overlap logic still accessed the slice.
	base64Only := strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 5)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-all-base64",
		SourceID:   "src-1",
		Content:    base64Only,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() should not panic or error, got: %v", err)
	}

	chunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatal("expected []*pipeline.Chunk")
	}

	// All content is non-text, so no chunks should survive
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for all-base64 content, got %d", len(chunks))
	}
}

func TestChunkerStage_NoInfiniteLoopOnMixedContent(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(1024), "chunk_overlap": float64(100)},
	}
	stage, _ := factory.Create(config, nil)

	// Simulate a real .api.mdx file: normal frontmatter, then a large base64 blob
	// with a space early in the chunk window (triggers word-boundary adjustment).
	// Before the fix, this caused an infinite loop because:
	// 1. Text chunks are appended (last StartOffset far behind)
	// 2. Base64 chunk is skipped by isLikelyNonText
	// 3. Word boundary sets end close to offset
	// 4. offset = end - overlap regresses backward
	// 5. Guard only checks last appended chunk, doesn't catch the regression
	normalText := strings.Repeat("This is normal markdown documentation with proper spacing and words. ", 40) // ~2800 chars
	// Base64-like content with a space near the start (triggers word-boundary + overlap regression)
	base64Block := "api: " + strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 40) // ~2400 chars
	trailingText := "\n" + strings.Repeat("More normal text after the base64 block for testing. ", 20)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-api-mdx",
		SourceID:   "src-docs",
		Content:    normalText + base64Block + trailingText,
	}

	type processResult struct {
		output any
		err    error
	}
	done := make(chan processResult, 1)
	go func() {
		out, err := stage.Process(context.Background(), input)
		done <- processResult{out, err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("Process() error = %v", res.err)
		}
		chunks := res.output.([]*pipeline.Chunk)
		if len(chunks) == 0 {
			t.Error("expected at least one chunk from the normal text portions")
		}
		for i, chunk := range chunks {
			if isLikelyNonText(chunk.Content) {
				t.Errorf("chunk %d contains non-text content that should have been filtered", i)
			}
		}
		t.Logf("completed with %d chunks (no infinite loop)", len(chunks))
	case <-time.After(3 * time.Second):
		t.Fatal("chunker stuck in infinite loop — did not complete within 3 seconds")
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
