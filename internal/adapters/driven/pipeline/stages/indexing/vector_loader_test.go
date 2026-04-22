package indexing

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// --- Test doubles ---

type stubVectorIndex struct {
	lastIDs         []string
	lastDocumentIDs []string
	lastSourceIDs   []string
	lastContents    []string
	lastEmbeddings  [][]float32
	indexCalled     bool
	shouldFail      bool
}

func (s *stubVectorIndex) Index(ctx context.Context, id string, documentID string, embedding []float32) error {
	return nil
}

func (s *stubVectorIndex) IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error {
	s.indexCalled = true
	s.lastIDs = ids
	s.lastDocumentIDs = documentIDs
	s.lastSourceIDs = sourceIDs
	s.lastContents = contents
	s.lastEmbeddings = embeddings
	if s.shouldFail {
		return &StageError{Stage: "test", Message: "forced failure"}
	}
	return nil
}

func (s *stubVectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	return nil, nil, nil
}

func (s *stubVectorIndex) SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string, documentFilter *domain.DocumentIDFilter) ([]driven.VectorSearchResult, error) {
	return nil, nil
}

func (s *stubVectorIndex) Delete(ctx context.Context, id string) error         { return nil }
func (s *stubVectorIndex) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (s *stubVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}
func (s *stubVectorIndex) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return nil
}
func (s *stubVectorIndex) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return nil
}
func (s *stubVectorIndex) HealthCheck(ctx context.Context) error { return nil }

// --- Factory tests ---

func TestVectorLoaderFactory_StageID(t *testing.T) {
	factory := NewVectorLoaderFactory()
	if factory.StageID() != VectorLoaderStageID {
		t.Errorf("StageID() = %q, want %q", factory.StageID(), VectorLoaderStageID)
	}
}

func TestVectorLoaderFactory_Descriptor(t *testing.T) {
	factory := NewVectorLoaderFactory()
	desc := factory.Descriptor()

	if desc.ID != VectorLoaderStageID {
		t.Errorf("Descriptor.ID = %q, want %q", desc.ID, VectorLoaderStageID)
	}
	if desc.Type != pipeline.StageTypeLoader {
		t.Errorf("Descriptor.Type = %v, want %v", desc.Type, pipeline.StageTypeLoader)
	}
	if desc.InputShape != pipeline.ShapeEmbeddedChunk {
		t.Errorf("Descriptor.InputShape = %v, want %v", desc.InputShape, pipeline.ShapeEmbeddedChunk)
	}
	if desc.OutputShape != pipeline.ShapeIndexedDoc {
		t.Errorf("Descriptor.OutputShape = %v, want %v", desc.OutputShape, pipeline.ShapeIndexedDoc)
	}
}

func TestVectorLoaderFactory_Validate(t *testing.T) {
	factory := NewVectorLoaderFactory()
	err := factory.Validate(pipeline.StageConfig{})
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestVectorLoaderFactory_Create(t *testing.T) {
	factory := NewVectorLoaderFactory()
	vectorIdx := &stubVectorIndex{}

	caps := pipeline.NewCapabilitySet()
	caps.Add(pipeline.CapabilityVectorStore, "test-vector-store", vectorIdx)

	stage, err := factory.Create(pipeline.StageConfig{}, caps)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	loaderStage, ok := stage.(*VectorLoaderStage)
	if !ok {
		t.Fatalf("Create() returned wrong type: %T", stage)
	}

	if loaderStage.vectorIndex != vectorIdx {
		t.Error("Create() did not set vectorIndex correctly")
	}
}

func TestVectorLoaderFactory_Create_MissingCapability(t *testing.T) {
	factory := NewVectorLoaderFactory()
	caps := pipeline.NewCapabilitySet()

	_, err := factory.Create(pipeline.StageConfig{}, caps)
	if err == nil {
		t.Fatal("Create() should error when vector_store capability is missing")
	}

	if !contains(err.Error(), "vector_store capability not available") {
		t.Errorf("Create() error = %q, want error containing 'vector_store capability not available'", err.Error())
	}
}

func TestVectorLoaderFactory_Create_InvalidInstanceType(t *testing.T) {
	factory := NewVectorLoaderFactory()
	caps := pipeline.NewCapabilitySet()

	// Add with wrong type
	caps.Add(pipeline.CapabilityVectorStore, "test-vector-store", "not a vector index")

	_, err := factory.Create(pipeline.StageConfig{}, caps)
	if err == nil {
		t.Fatal("Create() should error when instance type is invalid")
	}

	if !contains(err.Error(), "invalid vector_store instance type") {
		t.Errorf("Create() error = %q, want error containing 'invalid vector_store instance type'", err.Error())
	}
}

// --- VectorLoaderStage tests ---

func TestVectorLoaderStage_Process_PassesSourceIDs(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content one",
			Embedding:  []float32{0.1, 0.2, 0.3},
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content two",
			Embedding:  []float32{0.4, 0.5, 0.6},
		},
	}

	result, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if !vectorIdx.indexCalled {
		t.Fatal("IndexBatch was not called")
	}

	// Verify sourceIDs were passed correctly
	if len(vectorIdx.lastSourceIDs) != 2 {
		t.Fatalf("lastSourceIDs length = %d, want 2", len(vectorIdx.lastSourceIDs))
	}
	if vectorIdx.lastSourceIDs[0] != "source-A" {
		t.Errorf("lastSourceIDs[0] = %q, want 'source-A'", vectorIdx.lastSourceIDs[0])
	}
	if vectorIdx.lastSourceIDs[1] != "source-A" {
		t.Errorf("lastSourceIDs[1] = %q, want 'source-A'", vectorIdx.lastSourceIDs[1])
	}

	// Verify other arrays
	if len(vectorIdx.lastIDs) != 2 {
		t.Errorf("lastIDs length = %d, want 2", len(vectorIdx.lastIDs))
	}
	if len(vectorIdx.lastDocumentIDs) != 2 {
		t.Errorf("lastDocumentIDs length = %d, want 2", len(vectorIdx.lastDocumentIDs))
	}
	if len(vectorIdx.lastContents) != 2 {
		t.Errorf("lastContents length = %d, want 2", len(vectorIdx.lastContents))
	}
	if len(vectorIdx.lastEmbeddings) != 2 {
		t.Errorf("lastEmbeddings length = %d, want 2", len(vectorIdx.lastEmbeddings))
	}

	// Verify output
	output, ok := result.(*pipeline.IndexingOutput)
	if !ok {
		t.Fatalf("Process() returned wrong type: %T", result)
	}
	if output.DocumentID != "doc-1" {
		t.Errorf("output.DocumentID = %q, want 'doc-1'", output.DocumentID)
	}
	if len(output.ChunkIDs) != 2 {
		t.Errorf("output.ChunkIDs length = %d, want 2", len(output.ChunkIDs))
	}
}

func TestVectorLoaderStage_Process_MultipleSourceIDs(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content one",
			Embedding:  []float32{0.1},
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-2",
			SourceID:   "source-B",
			Content:    "content two",
			Embedding:  []float32{0.2},
		},
		{
			ID:         "chunk-3",
			DocumentID: "doc-3",
			SourceID:   "source-C",
			Content:    "content three",
			Embedding:  []float32{0.3},
		},
	}

	_, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(vectorIdx.lastSourceIDs) != 3 {
		t.Fatalf("lastSourceIDs length = %d, want 3", len(vectorIdx.lastSourceIDs))
	}
	if vectorIdx.lastSourceIDs[0] != "source-A" {
		t.Errorf("lastSourceIDs[0] = %q, want 'source-A'", vectorIdx.lastSourceIDs[0])
	}
	if vectorIdx.lastSourceIDs[1] != "source-B" {
		t.Errorf("lastSourceIDs[1] = %q, want 'source-B'", vectorIdx.lastSourceIDs[1])
	}
	if vectorIdx.lastSourceIDs[2] != "source-C" {
		t.Errorf("lastSourceIDs[2] = %q, want 'source-C'", vectorIdx.lastSourceIDs[2])
	}
}

func TestVectorLoaderStage_Process_EmptySourceID(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "", // Empty source ID
			Content:    "content",
			Embedding:  []float32{0.1},
		},
	}

	_, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(vectorIdx.lastSourceIDs) != 1 {
		t.Fatalf("lastSourceIDs length = %d, want 1", len(vectorIdx.lastSourceIDs))
	}
	if vectorIdx.lastSourceIDs[0] != "" {
		t.Errorf("lastSourceIDs[0] = %q, want empty string", vectorIdx.lastSourceIDs[0])
	}
}

func TestVectorLoaderStage_Process_EmptyChunks(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	result, err := stage.Process(context.Background(), []*pipeline.Chunk{})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if vectorIdx.indexCalled {
		t.Error("IndexBatch should not be called for empty chunks")
	}

	output, ok := result.(*pipeline.IndexingOutput)
	if !ok {
		t.Fatalf("Process() returned wrong type: %T", result)
	}
	if output.DocumentID != "" {
		t.Errorf("output.DocumentID = %q, want empty string", output.DocumentID)
	}
}

func TestVectorLoaderStage_Process_ChunksWithoutEmbeddings(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content",
			Embedding:  nil, // No embedding
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content",
			Embedding:  []float32{}, // Empty embedding
		},
	}

	result, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if vectorIdx.indexCalled {
		t.Error("IndexBatch should not be called when no chunks have embeddings")
	}

	output, ok := result.(*pipeline.IndexingOutput)
	if !ok {
		t.Fatalf("Process() returned wrong type: %T", result)
	}
	if len(output.ChunkIDs) != 0 {
		t.Errorf("output.ChunkIDs length = %d, want 0", len(output.ChunkIDs))
	}
}

func TestVectorLoaderStage_Process_MixedChunks(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content one",
			Embedding:  []float32{0.1, 0.2}, // Has embedding
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content two",
			Embedding:  nil, // No embedding
		},
		{
			ID:         "chunk-3",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content three",
			Embedding:  []float32{0.3, 0.4}, // Has embedding
		},
	}

	_, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Only chunks with embeddings should be indexed
	if len(vectorIdx.lastIDs) != 2 {
		t.Errorf("lastIDs length = %d, want 2", len(vectorIdx.lastIDs))
	}
	if len(vectorIdx.lastSourceIDs) != 2 {
		t.Errorf("lastSourceIDs length = %d, want 2", len(vectorIdx.lastSourceIDs))
	}

	// Verify correct chunks were indexed
	if vectorIdx.lastIDs[0] != "chunk-1" {
		t.Errorf("lastIDs[0] = %q, want 'chunk-1'", vectorIdx.lastIDs[0])
	}
	if vectorIdx.lastIDs[1] != "chunk-3" {
		t.Errorf("lastIDs[1] = %q, want 'chunk-3'", vectorIdx.lastIDs[1])
	}
}

func TestVectorLoaderStage_Process_InvalidInput(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	_, err := stage.Process(context.Background(), "invalid input")
	if err == nil {
		t.Fatal("Process() should error with invalid input type")
	}

	if !contains(err.Error(), "expected []*pipeline.Chunk") {
		t.Errorf("Process() error = %q, want error containing 'expected []*pipeline.Chunk'", err.Error())
	}
}

func TestVectorLoaderStage_Process_IndexBatchError(t *testing.T) {
	vectorIdx := &stubVectorIndex{shouldFail: true}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    "content",
			Embedding:  []float32{0.1},
		},
	}

	_, err := stage.Process(context.Background(), chunks)
	if err == nil {
		t.Fatal("Process() should error when IndexBatch fails")
	}

	if !contains(err.Error(), "failed to index embeddings") {
		t.Errorf("Process() error = %q, want error containing 'failed to index embeddings'", err.Error())
	}
}

func TestVectorLoaderStage_Process_InvalidUTF8Sanitization(t *testing.T) {
	vectorIdx := &stubVectorIndex{}
	stage := &VectorLoaderStage{
		descriptor:  NewVectorLoaderFactory().Descriptor(),
		vectorIndex: vectorIdx,
	}

	// Create content with invalid UTF-8
	invalidUTF8 := "Hello \xc3\x28 World"
	chunks := []*pipeline.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-A",
			Content:    invalidUTF8,
			Embedding:  []float32{0.1},
		},
	}

	_, err := stage.Process(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify the content was sanitized (invalid UTF-8 replaced)
	if len(vectorIdx.lastContents) != 1 {
		t.Fatalf("lastContents length = %d, want 1", len(vectorIdx.lastContents))
	}

	// The sanitized content should not contain invalid UTF-8
	// Original: "Hello \xc3\x28 World" -> Sanitized: "Hello ( World" or similar
	if vectorIdx.lastContents[0] == invalidUTF8 {
		t.Error("Process() should sanitize invalid UTF-8 content")
	}
}

func TestVectorLoaderStage_Descriptor(t *testing.T) {
	stage := &VectorLoaderStage{
		descriptor: NewVectorLoaderFactory().Descriptor(),
	}

	desc := stage.Descriptor()
	if desc.ID != VectorLoaderStageID {
		t.Errorf("Descriptor().ID = %q, want %q", desc.ID, VectorLoaderStageID)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
