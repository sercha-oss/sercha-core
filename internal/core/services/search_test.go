package services

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/runtime"
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

// createTestSearchService creates a SearchService with a mock executor for testing
// The mock executor delegates to the legacy searchEngine for actual search functionality
func createTestSearchService(searchEngine *mocks.MockSearchEngine, documentStore *mocks.MockDocumentStore, runtimeServices *runtime.Services) *searchService {
	// Create a mock executor that actually uses the search engine
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			// Perform actual search using the search engine
			opts := domain.SearchOptions{
				Mode:      domain.SearchModeTextOnly,
				Limit:     sctx.Pagination.Limit,
				Offset:    sctx.Pagination.Offset,
				SourceIDs: input.Filters.Sources,
			}
			// SearchEngine.Search requires a query embedding (nil for text-only search)
			chunks, totalCount, err := searchEngine.Search(ctx, input.Query, nil, opts)
			if err != nil {
				return nil, err
			}

			// Convert search engine results to pipeline results
			pipelineResults := make([]pipeline.PresentedResult, len(chunks))
			for i, rankedChunk := range chunks {
				pipelineResults[i] = pipeline.PresentedResult{
					DocumentID: rankedChunk.Chunk.DocumentID,
					ChunkID:    rankedChunk.Chunk.ID,
					SourceID:   rankedChunk.Chunk.SourceID,
					Title:      "",
					Snippet:    rankedChunk.Chunk.Content,
					Score:      rankedChunk.Score,
				}
			}

			return &pipeline.SearchOutput{
				Results:    pipelineResults,
				TotalCount: int64(totalCount),
				Timing: pipeline.ExecutionTiming{
					TotalMs: 10,
				},
			}, nil
		},
	}
	return NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default").(*searchService)
}

func TestSearchService_Search(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

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
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

	// Index a chunk
	chunk := &domain.Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	// Search with empty options - limit should default to 20
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify default limit is applied (20)
	if len(result.Results) > 20 {
		t.Errorf("expected max 20 results with default limit, got %d", len(result.Results))
	}
}

func TestSearchService_Search_LimitEnforcement(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

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
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

	// Save documents for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{ID: "doc-1", SourceID: "source-1", Title: "Doc 1"})
	_ = documentStore.Save(context.Background(), &domain.Document{ID: "doc-2", SourceID: "source-2", Title: "Doc 2"})
	_ = documentStore.Save(context.Background(), &domain.Document{ID: "doc-3", SourceID: "source-1", Title: "Doc 3"})

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
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

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

func TestSearchService_Search_Timing(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	embeddingService := mocks.NewMockEmbeddingService()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(embeddingService)
	svc := createTestSearchService(searchEngine, documentStore, runtimeServices)

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

// TestSearchService_Pagination_LimitOnly tests that limit is applied correctly
func TestSearchService_Pagination_LimitOnly(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 10 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 10)
			for i := 0; i < 10; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 10,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with limit=3
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify limit is applied: should return exactly 3 results
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results with limit=3, got %d", len(result.Results))
	}

	// Verify TotalCount reflects pre-pagination count
	if result.TotalCount != 10 {
		t.Errorf("expected TotalCount=10 (pre-pagination), got %d", result.TotalCount)
	}
}

// TestSearchService_Pagination_OffsetOnly tests that offset is applied correctly
func TestSearchService_Pagination_OffsetOnly(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 10 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 10)
			for i := 0; i < 10; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 10,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with offset=2
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:   domain.SearchModeTextOnly,
		Offset: 2,
		Limit:  20, // Set a high limit so we can verify offset behavior
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify offset is applied: should skip first 2 results, return 8
	if len(result.Results) != 8 {
		t.Errorf("expected 8 results with offset=2, got %d", len(result.Results))
	}

	// Verify TotalCount reflects pre-pagination count
	if result.TotalCount != 10 {
		t.Errorf("expected TotalCount=10 (pre-pagination), got %d", result.TotalCount)
	}
}

// TestSearchService_Pagination_LimitAndOffset tests that limit and offset work together
func TestSearchService_Pagination_LimitAndOffset(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 10 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 10)
			for i := 0; i < 10; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 10,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with offset=2, limit=3
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:   domain.SearchModeTextOnly,
		Offset: 2,
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify offset and limit together: skip 2, then take 3
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results with offset=2, limit=3, got %d", len(result.Results))
	}

	// Verify TotalCount reflects pre-pagination count
	if result.TotalCount != 10 {
		t.Errorf("expected TotalCount=10 (pre-pagination), got %d", result.TotalCount)
	}
}

// TestSearchService_Pagination_OffsetBeyondTotal tests offset beyond total results
func TestSearchService_Pagination_OffsetBeyondTotal(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 5 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 5)
			for i := 0; i < 5; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 5,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with offset=10 (beyond total of 5)
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:   domain.SearchModeTextOnly,
		Offset: 10,
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify empty results when offset is beyond total
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results with offset=10 beyond total=5, got %d", len(result.Results))
	}

	// Verify TotalCount still reflects pre-pagination count
	if result.TotalCount != 5 {
		t.Errorf("expected TotalCount=5 (pre-pagination), got %d", result.TotalCount)
	}
}

// TestSearchService_Pagination_DefaultBehavior tests default behavior with no pagination params
func TestSearchService_Pagination_DefaultBehavior(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 10 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 10)
			for i := 0; i < 10; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 10,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with limit=0, offset=0 (defaults)
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:   domain.SearchModeTextOnly,
		Limit:  0,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With limit=0, should apply default limit (20)
	// Since we have 10 results, all should be returned
	if len(result.Results) != 10 {
		t.Errorf("expected all 10 results with default limit, got %d", len(result.Results))
	}

	// Verify TotalCount reflects total results
	if result.TotalCount != 10 {
		t.Errorf("expected TotalCount=10, got %d", result.TotalCount)
	}
}

// TestSearchService_Pagination_OffsetAtBoundary tests offset exactly at the boundary
func TestSearchService_Pagination_OffsetAtBoundary(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock executor that returns 5 results
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			results := make([]pipeline.PresentedResult, 5)
			for i := 0; i < 5; i++ {
				results[i] = pipeline.PresentedResult{
					DocumentID: "doc-123",
					ChunkID:    generateID(),
					SourceID:   "source-456",
					Snippet:    "Test snippet",
					Score:      0.9,
				}
			}
			return &pipeline.SearchOutput{
				Results:    results,
				TotalCount: 5,
			}, nil
		},
	}

	svc := NewSearchService(nil, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{
		ID:       "doc-123",
		SourceID: "source-456",
		Title:    "Test Document",
	})

	// Search with offset=5 (exactly at total count)
	result, err := svc.Search(context.Background(), "Test", domain.SearchOptions{
		Mode:   domain.SearchModeTextOnly,
		Offset: 5,
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify empty results when offset equals total
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results with offset=5 at boundary of total=5, got %d", len(result.Results))
	}

	// Verify TotalCount still reflects pre-pagination count
	if result.TotalCount != 5 {
		t.Errorf("expected TotalCount=5 (pre-pagination), got %d", result.TotalCount)
	}
}
