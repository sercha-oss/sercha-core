package mocks

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// TestMockVectorIndex_InterfaceCompliance verifies MockVectorIndex implements driven.VectorIndex
func TestMockVectorIndex_InterfaceCompliance(t *testing.T) {
	var _ driven.VectorIndex = (*MockVectorIndex)(nil)
}

// TestMockSearchEngine_InterfaceCompliance verifies MockSearchEngine implements driven.SearchEngine
func TestMockSearchEngine_InterfaceCompliance(t *testing.T) {
	var _ driven.SearchEngine = (*MockSearchEngine)(nil)
}

// TestNewMockVectorIndex tests the constructor
func TestNewMockVectorIndex(t *testing.T) {
	mock := NewMockVectorIndex()
	if mock == nil {
		t.Fatal("NewMockVectorIndex() returned nil")
	}
	if mock.embeddings == nil {
		t.Error("embeddings map should be initialized")
	}
}

// TestMockVectorIndex_Index tests the Index method
func TestMockVectorIndex_Index(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	embedding := []float32{0.1, 0.2, 0.3}
	err := mock.Index(ctx, "test-id", "doc-1", embedding)
	if err != nil {
		t.Errorf("Index() error = %v", err)
	}

	// Verify embedding was stored
	stored, ok := mock.GetEmbedding("test-id")
	if !ok {
		t.Error("Embedding was not stored")
	}
	if len(stored) != len(embedding) {
		t.Errorf("Stored embedding length = %d, want %d", len(stored), len(embedding))
	}
}

// TestMockVectorIndex_IndexBatch tests the IndexBatch method
func TestMockVectorIndex_IndexBatch(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	ids := []string{"id1", "id2", "id3"}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
		{0.7, 0.8, 0.9},
	}

	docIDs := []string{"doc1", "doc2", "doc3"}
	sourceIDs := []string{"src1", "src2", "src3"}
	contents := []string{"content1", "content2", "content3"}
	err := mock.IndexBatch(ctx, ids, docIDs, sourceIDs, contents, embeddings)
	if err != nil {
		t.Errorf("IndexBatch() error = %v", err)
	}

	// Verify all embeddings were stored
	for i, id := range ids {
		stored, ok := mock.GetEmbedding(id)
		if !ok {
			t.Errorf("Embedding for %q was not stored", id)
			continue
		}
		if len(stored) != len(embeddings[i]) {
			t.Errorf("Stored embedding for %q length = %d, want %d", id, len(stored), len(embeddings[i]))
		}
	}

	// Verify count
	if mock.Count() != len(ids) {
		t.Errorf("Count() = %d, want %d", mock.Count(), len(ids))
	}
}

// TestMockVectorIndex_IndexBatch_UnequalLength tests handling of unequal ids and embeddings
func TestMockVectorIndex_IndexBatch_UnequalLength(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// More IDs than embeddings
	ids := []string{"id1", "id2", "id3"}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
	}

	docIDs := []string{"doc1", "doc2", "doc3"}
	sourceIDs := []string{"src1", "src2", "src3"}
	contents := []string{"content1", "content2", "content3"}
	// The mock silently handles this - it only stores what it can
	err := mock.IndexBatch(ctx, ids, docIDs, sourceIDs, contents, embeddings)
	if err != nil {
		t.Errorf("IndexBatch() error = %v", err)
	}

	// Only first ID should have an embedding
	if mock.Count() != 1 {
		t.Errorf("Count() = %d, want 1", mock.Count())
	}
}

// TestMockVectorIndex_Search tests the Search method
func TestMockVectorIndex_Search(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index some embeddings first
	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1, 0.2, 0.3})
	_ = mock.Index(ctx, "id2", "doc1", []float32{0.4, 0.5, 0.6})
	_ = mock.Index(ctx, "id3", "doc2", []float32{0.7, 0.8, 0.9})

	// Search
	queryEmbedding := []float32{0.1, 0.2, 0.3}
	ids, distances, err := mock.Search(ctx, queryEmbedding, 2)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	if len(ids) > 2 {
		t.Errorf("Search() returned %d results, want at most 2", len(ids))
	}
	if len(ids) != len(distances) {
		t.Errorf("ids and distances length mismatch: %d vs %d", len(ids), len(distances))
	}

	// Mock returns 0.0 distances
	for _, d := range distances {
		if d != 0.0 {
			t.Errorf("Mock distance = %v, want 0.0", d)
		}
	}
}

// TestMockVectorIndex_Search_EmptyIndex tests search on empty index
func TestMockVectorIndex_Search_EmptyIndex(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	ids, distances, err := mock.Search(ctx, []float32{0.1, 0.2}, 10)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Search() on empty index returned %d results, want 0", len(ids))
	}
	if len(distances) != 0 {
		t.Errorf("Search() on empty index returned %d distances, want 0", len(distances))
	}
}

// TestMockVectorIndex_Search_KLargerThanIndex tests search with k larger than index size
func TestMockVectorIndex_Search_KLargerThanIndex(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1})
	_ = mock.Index(ctx, "id2", "doc1", []float32{0.2})

	ids, distances, err := mock.Search(ctx, []float32{0.1}, 100)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}
	// Should return at most what's available
	if len(ids) > 2 {
		t.Errorf("Search() returned %d results, want at most 2", len(ids))
	}
	if len(ids) != len(distances) {
		t.Errorf("ids and distances length mismatch: %d vs %d", len(ids), len(distances))
	}
}

// TestMockVectorIndex_Delete tests the Delete method
func TestMockVectorIndex_Delete(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index an embedding
	_ = mock.Index(ctx, "test-id", "doc1", []float32{0.1, 0.2, 0.3})
	if mock.Count() != 1 {
		t.Fatalf("Count() after Index = %d, want 1", mock.Count())
	}

	// Delete it
	err := mock.Delete(ctx, "test-id")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify it's gone
	if mock.Count() != 0 {
		t.Errorf("Count() after Delete = %d, want 0", mock.Count())
	}
	_, ok := mock.GetEmbedding("test-id")
	if ok {
		t.Error("Embedding should be deleted")
	}
}

// TestMockVectorIndex_Delete_NonExistent tests deleting non-existent ID
func TestMockVectorIndex_Delete_NonExistent(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Delete non-existent ID should not error
	err := mock.Delete(ctx, "non-existent")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

// TestMockVectorIndex_DeleteBatch tests the DeleteBatch method
func TestMockVectorIndex_DeleteBatch(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index some embeddings
	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1})
	_ = mock.Index(ctx, "id2", "doc1", []float32{0.2})
	_ = mock.Index(ctx, "id3", "doc2", []float32{0.3})
	if mock.Count() != 3 {
		t.Fatalf("Count() after Index = %d, want 3", mock.Count())
	}

	// Delete batch
	err := mock.DeleteBatch(ctx, []string{"id1", "id3"})
	if err != nil {
		t.Errorf("DeleteBatch() error = %v", err)
	}

	// Verify correct items were deleted
	if mock.Count() != 1 {
		t.Errorf("Count() after DeleteBatch = %d, want 1", mock.Count())
	}
	_, ok := mock.GetEmbedding("id2")
	if !ok {
		t.Error("id2 should still exist")
	}
}

// TestMockVectorIndex_DeleteBatch_Empty tests deleting empty batch
func TestMockVectorIndex_DeleteBatch_Empty(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1})

	err := mock.DeleteBatch(ctx, []string{})
	if err != nil {
		t.Errorf("DeleteBatch() with empty ids error = %v", err)
	}

	// Count should be unchanged
	if mock.Count() != 1 {
		t.Errorf("Count() after empty DeleteBatch = %d, want 1", mock.Count())
	}
}

// TestMockVectorIndex_HealthCheck tests the HealthCheck method
func TestMockVectorIndex_HealthCheck(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	err := mock.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

// TestMockVectorIndex_Reset tests the Reset method
func TestMockVectorIndex_Reset(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index some embeddings
	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1})
	_ = mock.Index(ctx, "id2", "doc1", []float32{0.2})
	if mock.Count() != 2 {
		t.Fatalf("Count() after Index = %d, want 2", mock.Count())
	}

	// Reset
	mock.Reset()

	// Verify all data is cleared
	if mock.Count() != 0 {
		t.Errorf("Count() after Reset = %d, want 0", mock.Count())
	}
}

// TestMockVectorIndex_GetEmbedding tests the GetEmbedding helper method
func TestMockVectorIndex_GetEmbedding(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Test non-existent
	_, ok := mock.GetEmbedding("non-existent")
	if ok {
		t.Error("GetEmbedding() for non-existent ID should return false")
	}

	// Index and retrieve
	embedding := []float32{0.1, 0.2, 0.3}
	_ = mock.Index(ctx, "test-id", "doc1", embedding)

	retrieved, ok := mock.GetEmbedding("test-id")
	if !ok {
		t.Error("GetEmbedding() for existing ID should return true")
	}
	if len(retrieved) != len(embedding) {
		t.Errorf("GetEmbedding() length = %d, want %d", len(retrieved), len(embedding))
	}
	for i, v := range retrieved {
		if v != embedding[i] {
			t.Errorf("GetEmbedding()[%d] = %v, want %v", i, v, embedding[i])
		}
	}
}

// TestMockVectorIndex_Count tests the Count helper method
func TestMockVectorIndex_Count(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	if mock.Count() != 0 {
		t.Errorf("Count() on new mock = %d, want 0", mock.Count())
	}

	_ = mock.Index(ctx, "id1", "doc1", []float32{0.1})
	if mock.Count() != 1 {
		t.Errorf("Count() after 1 Index = %d, want 1", mock.Count())
	}

	_ = mock.Index(ctx, "id2", "doc1", []float32{0.2})
	if mock.Count() != 2 {
		t.Errorf("Count() after 2 Index = %d, want 2", mock.Count())
	}

	_ = mock.Delete(ctx, "id1")
	if mock.Count() != 1 {
		t.Errorf("Count() after Delete = %d, want 1", mock.Count())
	}
}

// TestMockVectorIndex_ConcurrentAccess tests thread safety of the mock
func TestMockVectorIndex_ConcurrentAccess(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Run concurrent operations
	done := make(chan bool, 3)

	// Writer goroutine 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = mock.Index(ctx, "id1", "doc1", []float32{float32(i)})
		}
		done <- true
	}()

	// Writer goroutine 2
	go func() {
		for i := 0; i < 100; i++ {
			_ = mock.Index(ctx, "id2", "doc1", []float32{float32(i)})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _, _ = mock.Search(ctx, []float32{0.1}, 10)
			mock.Count()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Just verify it didn't panic or deadlock
	t.Log("Concurrent access test completed without deadlock")
}

// TestMockVectorIndex_IndexOverwrite tests that indexing same ID overwrites
func TestMockVectorIndex_IndexOverwrite(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index first embedding
	_ = mock.Index(ctx, "test-id", "doc1", []float32{0.1, 0.2})

	// Overwrite with new embedding
	_ = mock.Index(ctx, "test-id", "doc1", []float32{0.3, 0.4, 0.5})

	// Count should still be 1
	if mock.Count() != 1 {
		t.Errorf("Count() after overwrite = %d, want 1", mock.Count())
	}

	// Should have new embedding
	retrieved, ok := mock.GetEmbedding("test-id")
	if !ok {
		t.Fatal("Embedding should exist after overwrite")
	}
	if len(retrieved) != 3 {
		t.Errorf("Overwritten embedding length = %d, want 3", len(retrieved))
	}
}
