package services

import (
	"context"
	"testing"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven/mocks"
	"github.com/custodia-labs/sercha-core/internal/runtime"
)

// createTestServices creates runtime services for testing
func createTestServices(embeddingService *mocks.MockEmbeddingService) *runtime.Services {
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	if embeddingService != nil {
		services.SetEmbeddingService(embeddingService)
	}
	return services
}

func TestSearchService_Search(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index some chunks
	doc := &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	}
	_ = documentStore.Save(context.Background(), doc)

	chunks := []*domain.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-123",
			SourceID:   "source-456",
			Content:    "This is a test document about Go programming",
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-123",
			SourceID:   "source-456",
			Content:    "Another chunk about Python programming",
		},
		{
			ID:         "chunk-3",
			DocumentID: "doc-123",
			SourceID:   "source-456",
			Content:    "JavaScript is also a programming language",
		},
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Search for "Go programming"
	result, err := svc.Search(context.Background(), "Go", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Query != "Go" {
		t.Errorf("expected query 'Go', got %s", result.Query)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result for 'Go', got %d", len(result.Results))
	}

	// Search for "programming" (should match all)
	result, err = svc.Search(context.Background(), "programming", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results for 'programming', got %d", len(result.Results))
	}

	// Search with no matches
	result, err = svc.Search(context.Background(), "nonexistent", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(result.Results))
	}
}

func TestSearchService_Search_DefaultOptions(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index a chunk
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Search with empty options
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mode should default to hybrid (but fall back to text-only if embedding fails)
	if result.Mode != domain.SearchModeHybrid && result.Mode != domain.SearchModeTextOnly {
		t.Errorf("unexpected mode: %s", result.Mode)
	}
}

func TestSearchService_Search_LimitEnforcement(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index many chunks
	chunks := make([]*domain.Chunk, 150)
	for i := 0; i < 150; i++ {
		chunks[i] = &domain.Chunk{
			ID:         generateID(),
			DocumentID: "doc-123",
			SourceID:   "source-456",
			Content:    "Test content for searching",
		}
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Search with limit > 100 (should be capped)
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) > 100 {
		t.Errorf("expected at most 100 results, got %d", len(result.Results))
	}

	// Search with 0 limit (should default to 20)
	result, err = svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) > 20 {
		t.Errorf("expected at most 20 results with default limit, got %d", len(result.Results))
	}
}

func TestSearchService_SearchBySource(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index chunks for different sources
	chunks := []*domain.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-1",
			Content:    "Test content for source 1",
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-2",
			SourceID:   "source-2",
			Content:    "Test content for source 2",
		},
		{
			ID:         "chunk-3",
			DocumentID: "doc-3",
			SourceID:   "source-1",
			Content:    "More test content for source 1",
		},
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Search in source-1 only
	result, err := svc.SearchBySource(context.Background(), "source-1", "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results for source-1, got %d", len(result.Results))
	}

	// Search in source-2 only
	result, err = svc.SearchBySource(context.Background(), "source-2", "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result for source-2, got %d", len(result.Results))
	}
}

func TestSearchService_Search_HybridMode(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index a chunk
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content for hybrid search",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Search in hybrid mode
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be returned (mode may fall back to text if embedding fails)
	if result == nil {
		t.Error("expected result to be returned")
	}
}

func TestSearchService_Search_EmbeddingFallback(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Configure embedding service to fail
	embeddingService.SetFailNext(true)

	// Index a chunk
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Search in hybrid mode (should fall back to text-only)
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mode != domain.SearchModeTextOnly {
		t.Errorf("expected mode to fall back to text-only, got %s", result.Mode)
	}
}

func TestSearchService_Search_SemanticOnlyWithoutEmbedding(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	// No embedding service - pass nil to createTestServices
	runtimeServices := createTestServices(nil)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index a chunk to ensure search can run
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Try semantic-only search without embedding service
	// Should degrade to text-only since embedding not available
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeSemanticOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mode should have degraded to text-only
	if result.Mode != domain.SearchModeTextOnly {
		t.Errorf("expected mode to degrade to text-only, got %s", result.Mode)
	}
}

func TestSearchService_Suggest(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Suggest should return empty for now (not implemented)
	suggestions, err := svc.Suggest(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suggestions == nil {
		t.Error("expected suggestions to be non-nil")
	}
}

func TestSearchService_Search_Timing(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index a chunk
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Search and check timing
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Took <= 0 {
		t.Error("expected Took to be positive")
	}
}
