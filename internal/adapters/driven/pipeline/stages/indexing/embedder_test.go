package indexing

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// mockEmbeddingService is a mock implementation of driven.EmbeddingService
type mockEmbeddingService struct {
	embedFunc      func(ctx context.Context, texts []string) ([][]float32, error)
	dimensions     int
	model          string
	healthCheckErr error
}

func (m *mockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts)
	}
	// Default behavior: return embeddings of the same length as input
	embeddings := make([][]float32, len(texts))
	for i := range embeddings {
		embeddings[i] = make([]float32, m.dimensions)
		// Fill with some dummy values
		for j := range embeddings[i] {
			embeddings[i][j] = float32(i + j)
		}
	}
	return embeddings, nil
}

func (m *mockEmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	embedding := make([]float32, m.dimensions)
	return embedding, nil
}

func (m *mockEmbeddingService) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbeddingService) Model() string {
	return m.model
}

func (m *mockEmbeddingService) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *mockEmbeddingService) Close() error {
	return nil
}

func TestEmbedderFactory_Create_ReturnsNoOpWhenEmbedderMissing(t *testing.T) {
	factory := NewEmbedderFactory()

	// Create with empty capability set (no embedder)
	capabilities := pipeline.NewCapabilitySet()
	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if stage == nil {
		t.Fatal("expected stage to be created")
	}

	// Verify it's a NoOpEmbedderStage
	_, ok := stage.(*NoOpEmbedderStage)
	if !ok {
		t.Errorf("expected NoOpEmbedderStage, got %T", stage)
	}
}

func TestEmbedderFactory_Create_ReturnsEmbedderStageWhenEmbedderPresent(t *testing.T) {
	factory := NewEmbedderFactory()

	// Create capability set with embedder
	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if stage == nil {
		t.Fatal("expected stage to be created")
	}

	// Verify it's an EmbedderStage
	embedderStage, ok := stage.(*EmbedderStage)
	if !ok {
		t.Errorf("expected EmbedderStage, got %T", stage)
	}

	if embedderStage.embedder == nil {
		t.Error("expected embedder to be set")
	}
}

func TestEmbedderFactory_Create_WithBatchSizeParameter(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
		Parameters: map[string]any{
			"batch_size": float64(64),
		},
	}

	stage, err := factory.Create(config, capabilities)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	embedderStage, ok := stage.(*EmbedderStage)
	if !ok {
		t.Fatalf("expected EmbedderStage, got %T", stage)
	}

	if embedderStage.batchSize != 64 {
		t.Errorf("expected batch size 64, got %d", embedderStage.batchSize)
	}
}

func TestEmbedderFactory_Create_InvalidEmbedderInstanceType(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	// Add a capability with wrong type
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", "not-an-embedder")

	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
	}

	_, err := factory.Create(config, capabilities)

	if err == nil {
		t.Error("expected error for invalid embedder instance type")
	}
}

func TestEmbedderFactory_Descriptor(t *testing.T) {
	factory := NewEmbedderFactory()
	desc := factory.Descriptor()

	if desc.ID != EmbedderStageID {
		t.Errorf("expected ID %s, got %s", EmbedderStageID, desc.ID)
	}

	if desc.Type != pipeline.StageTypeEnricher {
		t.Errorf("expected type Enricher, got %s", desc.Type)
	}

	if desc.InputShape != pipeline.ShapeChunk {
		t.Errorf("expected input shape Chunk, got %s", desc.InputShape)
	}

	if desc.OutputShape != pipeline.ShapeEmbeddedChunk {
		t.Errorf("expected output shape EmbeddedChunk, got %s", desc.OutputShape)
	}

	// Verify capability is optional
	if len(desc.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(desc.Capabilities))
	}

	if desc.Capabilities[0].Type != pipeline.CapabilityEmbedder {
		t.Errorf("expected embedder capability, got %s", desc.Capabilities[0].Type)
	}

	if desc.Capabilities[0].Mode != pipeline.CapabilityOptional {
		t.Errorf("expected optional capability, got %s", desc.Capabilities[0].Mode)
	}
}

func TestNoOpEmbedderStage_Process_PassesChunksUnchanged(t *testing.T) {
	factory := NewEmbedderFactory()
	capabilities := pipeline.NewCapabilitySet() // Empty - no embedder

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// Create test chunks
	chunks := []*pipeline.Chunk{
		{
			ID:          "chunk-1",
			DocumentID:  "doc-1",
			Content:     "This is the first chunk",
			Position:    0,
			StartOffset: 0,
			EndOffset:   23,
			Embedding:   nil, // No embedding initially
		},
		{
			ID:          "chunk-2",
			DocumentID:  "doc-1",
			Content:     "This is the second chunk",
			Position:    1,
			StartOffset: 23,
			EndOffset:   47,
			Embedding:   nil,
		},
	}

	result, err := stage.Process(context.Background(), chunks)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultChunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatalf("expected []*pipeline.Chunk, got %T", result)
	}

	if len(resultChunks) != len(chunks) {
		t.Errorf("expected %d chunks, got %d", len(chunks), len(resultChunks))
	}

	// Verify chunks are unchanged
	for i, chunk := range resultChunks {
		if chunk.ID != chunks[i].ID {
			t.Errorf("chunk %d: expected ID %s, got %s", i, chunks[i].ID, chunk.ID)
		}
		if chunk.Content != chunks[i].Content {
			t.Errorf("chunk %d: expected content %s, got %s", i, chunks[i].Content, chunk.Content)
		}
		if chunk.Embedding != nil {
			t.Errorf("chunk %d: expected no embedding, got %v", i, chunk.Embedding)
		}
	}
}

func TestNoOpEmbedderStage_Process_EmptyChunks(t *testing.T) {
	factory := NewEmbedderFactory()
	capabilities := pipeline.NewCapabilitySet()

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	chunks := []*pipeline.Chunk{}
	result, err := stage.Process(context.Background(), chunks)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultChunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatalf("expected []*pipeline.Chunk, got %T", result)
	}

	if len(resultChunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(resultChunks))
	}
}

func TestNoOpEmbedderStage_Process_InvalidInput(t *testing.T) {
	factory := NewEmbedderFactory()
	capabilities := pipeline.NewCapabilitySet()

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// Test with invalid input type
	_, err = stage.Process(context.Background(), "invalid input")
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

func TestEmbedderStage_Process_GeneratesEmbeddings(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	chunks := []*pipeline.Chunk{
		{
			ID:          "chunk-1",
			DocumentID:  "doc-1",
			Content:     "First chunk content",
			Position:    0,
			StartOffset: 0,
			EndOffset:   19,
		},
		{
			ID:          "chunk-2",
			DocumentID:  "doc-1",
			Content:     "Second chunk content",
			Position:    1,
			StartOffset: 19,
			EndOffset:   39,
		},
	}

	result, err := stage.Process(context.Background(), chunks)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultChunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatalf("expected []*pipeline.Chunk, got %T", result)
	}

	// Verify embeddings were added
	for i, chunk := range resultChunks {
		if chunk.Embedding == nil {
			t.Errorf("chunk %d: expected embedding to be set", i)
		}
		if len(chunk.Embedding) != mockEmbedder.Dimensions() {
			t.Errorf("chunk %d: expected embedding dimension %d, got %d", i, mockEmbedder.Dimensions(), len(chunk.Embedding))
		}
	}
}

func TestEmbedderStage_Process_EmptyChunks(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	chunks := []*pipeline.Chunk{}
	result, err := stage.Process(context.Background(), chunks)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultChunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatalf("expected []*pipeline.Chunk, got %T", result)
	}

	if len(resultChunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(resultChunks))
	}
}

func TestEmbedderStage_Process_InvalidInput(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// Test with invalid input type
	_, err = stage.Process(context.Background(), "invalid input")
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

func TestEmbedderStage_Process_BatchProcessing(t *testing.T) {
	factory := NewEmbedderFactory()

	var (
		mu        sync.Mutex
		callCount int
	)
	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			embeddings := make([][]float32, len(texts))
			for i := range embeddings {
				embeddings[i] = make([]float32, 384)
			}
			return embeddings, nil
		},
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	config := pipeline.StageConfig{
		StageID: EmbedderStageID,
		Enabled: true,
		Parameters: map[string]any{
			"batch_size": float64(2),
		},
	}

	stage, err := factory.Create(config, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// Create 5 chunks, which should be processed in 3 batches (2, 2, 1)
	chunks := make([]*pipeline.Chunk, 5)
	for i := 0; i < 5; i++ {
		chunks[i] = &pipeline.Chunk{
			ID:          "chunk-" + string(rune('1'+i)),
			DocumentID:  "doc-1",
			Content:     "Content " + string(rune('1'+i)),
			Position:    i,
			StartOffset: i * 10,
			EndOffset:   (i + 1) * 10,
		}
	}

	result, err := stage.Process(context.Background(), chunks)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	resultChunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatalf("expected []*pipeline.Chunk, got %T", result)
	}

	// Verify all chunks have embeddings
	if len(resultChunks) != 5 {
		t.Errorf("expected 5 chunks, got %d", len(resultChunks))
	}

	// Verify batch processing: 5 chunks with batch size 2 should result in 3 calls
	expectedCalls := 3
	mu.Lock()
	gotCalls := callCount
	mu.Unlock()
	if gotCalls != expectedCalls {
		t.Errorf("expected %d embedder calls, got %d", expectedCalls, gotCalls)
	}

	for i, chunk := range resultChunks {
		if chunk.Embedding == nil {
			t.Errorf("chunk %d: expected embedding to be set", i)
		}
	}
}

func TestEmbedderStage_Process_EmbeddingServiceError(t *testing.T) {
	factory := NewEmbedderFactory()

	capabilities := pipeline.NewCapabilitySet()
	mockEmbedder := &mockEmbeddingService{
		dimensions: 384,
		model:      "test-model",
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			return nil, errors.New("embedding service error")
		},
	}
	capabilities.Add(pipeline.CapabilityEmbedder, "test-embedder", mockEmbedder)

	stage, err := factory.Create(pipeline.StageConfig{}, capabilities)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	chunks := []*pipeline.Chunk{
		{
			ID:          "chunk-1",
			DocumentID:  "doc-1",
			Content:     "Test content",
			Position:    0,
			StartOffset: 0,
			EndOffset:   12,
		},
	}

	_, err = stage.Process(context.Background(), chunks)

	if err == nil {
		t.Error("expected error from embedding service")
	}
}
