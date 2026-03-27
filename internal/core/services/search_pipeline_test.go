package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven/mocks"
)

// mockSearchExecutor is a mock implementation of SearchExecutor for testing
type mockSearchExecutor struct {
	executeFn    func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error)
	executeCount int
}

func (m *mockSearchExecutor) Execute(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
	m.executeCount++
	if m.executeFn != nil {
		return m.executeFn(ctx, sctx, input)
	}
	// Default implementation returns sample results
	return &pipeline.SearchOutput{
		Results: []pipeline.PresentedResult{
			{
				DocumentID: "doc-1",
				ChunkID:    "chunk-1",
				SourceID:   "source-1",
				Title:      "Test Result",
				Snippet:    "Test snippet",
				Score:      0.9,
			},
		},
		TotalCount: 1,
		Timing: pipeline.ExecutionTiming{
			TotalMs: 10,
		},
	}, nil
}

// TestSearchService_WithSearchExecutor tests that SearchService uses pipeline executor when provided
func TestSearchService_WithSearchExecutor(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock search executor
	executor := &mockSearchExecutor{}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Save a document for enrichment
	doc := &domain.Document{
		ID:       "doc-1",
		SourceID: "source-1",
		Title:    "Test Document",
	}
	_ = documentStore.Save(context.Background(), doc)

	// Perform search
	result, err := svc.Search(context.Background(), "test query", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify pipeline executor was called
	if executor.executeCount != 1 {
		t.Errorf("expected pipeline executor to be called once, got %d calls", executor.executeCount)
	}

	// Verify results were returned
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
	if result.TotalCount != 1 {
		t.Errorf("expected TotalCount=1, got %d", result.TotalCount)
	}
	if result.Query != "test query" {
		t.Errorf("expected query='test query', got %s", result.Query)
	}
}

// TestSearchService_SearchExecutorFallback tests fallback to legacy search when executor fails
func TestSearchService_SearchExecutorFallback(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock search executor that fails
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			return nil, errors.New("pipeline search failed")
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Index some chunks for legacy search to find
	chunks := []*domain.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-1",
			Content:    "Test content for fallback",
		},
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Perform search - should fall back to legacy
	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from fallback")
	}

	// Verify pipeline executor was attempted
	if executor.executeCount != 1 {
		t.Errorf("expected pipeline executor to be attempted once, got %d calls", executor.executeCount)
	}

	// Verify fallback worked
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result from fallback, got %d", len(result.Results))
	}
}

// TestSearchService_NilExecutorUsesLegacy tests that nil executor uses legacy search
func TestSearchService_NilExecutorUsesLegacy(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create service with nil executor
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index some chunks
	chunks := []*domain.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-1",
			Content:    "Test content",
		},
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Perform search
	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify results were returned via legacy
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result from legacy, got %d", len(result.Results))
	}
}

// TestSearchWithPipeline_Success tests successful pipeline search execution
func TestSearchWithPipeline_Success(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock search executor
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			return &pipeline.SearchOutput{
				Results: []pipeline.PresentedResult{
					{
						DocumentID: "doc-1",
						ChunkID:    "chunk-1",
						SourceID:   "source-1",
						Title:      "Result 1",
						Snippet:    "Snippet 1",
						Score:      0.95,
					},
					{
						DocumentID: "doc-2",
						ChunkID:    "chunk-2",
						SourceID:   "source-1",
						Title:      "Result 2",
						Snippet:    "Snippet 2",
						Score:      0.85,
					},
				},
				TotalCount: 2,
				Timing: pipeline.ExecutionTiming{
					TotalMs: 15,
				},
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Save documents for enrichment
	doc1 := &domain.Document{ID: "doc-1", SourceID: "source-1", Title: "Document 1"}
	doc2 := &domain.Document{ID: "doc-2", SourceID: "source-1", Title: "Document 2"}
	_ = documentStore.Save(context.Background(), doc1)
	_ = documentStore.Save(context.Background(), doc2)

	startTime := time.Now()
	result, err := svc.Search(context.Background(), "test query", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify results
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if result.TotalCount != 2 {
		t.Errorf("expected TotalCount=2, got %d", result.TotalCount)
	}
	if result.Query != "test query" {
		t.Errorf("expected query='test query', got %s", result.Query)
	}

	// Verify timing was captured
	if result.Took <= 0 {
		t.Error("expected Took to be positive")
	}
	if result.Took > time.Since(startTime) {
		t.Error("result.Took should not exceed actual time")
	}

	// Verify results are properly formatted
	if result.Results[0].Score != 0.95 {
		t.Errorf("expected first result score=0.95, got %f", result.Results[0].Score)
	}
	if result.Results[0].Chunk.Content != "Snippet 1" {
		t.Errorf("expected snippet to be in chunk content, got %s", result.Results[0].Chunk.Content)
	}
}

// TestSearchWithPipeline_SourceFilter tests that source filters are passed correctly
func TestSearchWithPipeline_SourceFilter(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Track what filters were passed to executor
	var capturedFilters pipeline.SearchFilters
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedFilters = input.Filters
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Search with source filter
	_, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:      domain.SearchModeHybrid,
		Limit:     10,
		SourceIDs: []string{"source-1", "source-2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify filters were passed correctly
	if len(capturedFilters.Sources) != 2 {
		t.Errorf("expected 2 source filters, got %d", len(capturedFilters.Sources))
	}
	if capturedFilters.Sources[0] != "source-1" || capturedFilters.Sources[1] != "source-2" {
		t.Errorf("expected sources [source-1, source-2], got %v", capturedFilters.Sources)
	}
}

// TestSearchWithPipeline_Pagination tests that pagination options are passed correctly
func TestSearchWithPipeline_Pagination(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Track what pagination was passed to executor
	var capturedPagination pipeline.PaginationConfig
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedPagination = sctx.Pagination
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Search with pagination
	_, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:   domain.SearchModeHybrid,
		Limit:  25,
		Offset: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pagination was passed correctly
	if capturedPagination.Limit != 25 {
		t.Errorf("expected limit=25, got %d", capturedPagination.Limit)
	}
	if capturedPagination.Offset != 10 {
		t.Errorf("expected offset=10, got %d", capturedPagination.Offset)
	}
}

// TestSearchWithPipeline_DocumentEnrichment tests that results are enriched with document data
func TestSearchWithPipeline_DocumentEnrichment(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			return &pipeline.SearchOutput{
				Results: []pipeline.PresentedResult{
					{
						DocumentID: "doc-1",
						ChunkID:    "chunk-1",
						SourceID:   "source-1",
						Snippet:    "Test snippet",
						Score:      0.9,
					},
				},
				TotalCount: 1,
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	// Save document
	doc := &domain.Document{
		ID:       "doc-1",
		SourceID: "source-1",
		Title:    "Full Document Title",
		Path:     "/path/to/doc",
	}
	_ = documentStore.Save(context.Background(), doc)

	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify document was enriched
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].Document == nil {
		t.Fatal("expected document to be enriched")
	}
	if result.Results[0].Document.Title != "Full Document Title" {
		t.Errorf("expected enriched document title, got %s", result.Results[0].Document.Title)
	}
	if result.Results[0].Document.Path != "/path/to/doc" {
		t.Errorf("expected enriched document path, got %s", result.Results[0].Document.Path)
	}
}

// TestSearchWithPipeline_ContextPassing tests that correct context is passed to executor
func TestSearchWithPipeline_ContextPassing(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Track what context was sent to executor
	var capturedContext *pipeline.SearchContext
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedContext = sctx
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}
	capSet := &pipeline.CapabilitySet{}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capSet)

	_, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify context was passed correctly
	if capturedContext == nil {
		t.Fatal("executor was not called with context")
	}
	if capturedContext.PipelineID != "default-search" {
		t.Errorf("expected PipelineID='default-search', got %s", capturedContext.PipelineID)
	}
	if capturedContext.Capabilities != capSet {
		t.Error("expected capabilities to be passed through")
	}
}

// TestSearchWithPipeline_EmptyResults tests handling of empty results
func TestSearchWithPipeline_EmptyResults(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
				Timing: pipeline.ExecutionTiming{
					TotalMs: 5,
				},
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	result, err := svc.Search(context.Background(), "nonexistent", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify empty results are handled correctly
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Results))
	}
	if result.TotalCount != 0 {
		t.Errorf("expected TotalCount=0, got %d", result.TotalCount)
	}
	if result.Took <= 0 {
		t.Error("expected Took to be positive even with empty results")
	}
}

// TestSearchBySource_WithPipeline tests SearchBySource with pipeline executor
func TestSearchBySource_WithPipeline(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Track what filters were passed
	var capturedFilters pipeline.SearchFilters
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedFilters = input.Filters
			return &pipeline.SearchOutput{
				Results: []pipeline.PresentedResult{
					{
						DocumentID: "doc-1",
						ChunkID:    "chunk-1",
						SourceID:   "source-1",
						Snippet:    "Test",
						Score:      0.9,
					},
				},
				TotalCount: 1,
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	result, err := svc.SearchBySource(context.Background(), "source-1", "test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify source filter was applied
	if len(capturedFilters.Sources) != 1 {
		t.Errorf("expected 1 source filter, got %d", len(capturedFilters.Sources))
	}
	if capturedFilters.Sources[0] != "source-1" {
		t.Errorf("expected source filter 'source-1', got %s", capturedFilters.Sources[0])
	}

	// Verify results
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
}

// TestSearchWithLegacy_BackwardCompatibility tests that legacy search still works
func TestSearchWithLegacy_BackwardCompatibility(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create service without executor - should use legacy
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, nil, nil)

	// Index chunks using legacy search engine
	doc := &domain.Document{
		ID:       "doc-1",
		SourceID: "source-1",
		Title:    "Test Document",
	}
	_ = documentStore.Save(context.Background(), doc)

	chunks := []*domain.Chunk{
		{
			ID:         "chunk-1",
			DocumentID: "doc-1",
			SourceID:   "source-1",
			Content:    "This is test content for legacy search",
		},
		{
			ID:         "chunk-2",
			DocumentID: "doc-1",
			SourceID:   "source-1",
			Content:    "More test content",
		},
	}
	_ = searchEngine.Index(context.Background(), chunks)

	// Perform search
	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify legacy search works
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results from legacy, got %d", len(result.Results))
	}
	if result.Query != "test" {
		t.Errorf("expected query='test', got %s", result.Query)
	}
	if result.Took <= 0 {
		t.Error("expected Took to be positive")
	}
}

// TestSearchWithPipeline_ResultMapping tests proper mapping from pipeline results to domain results
func TestSearchWithPipeline_ResultMapping(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			return &pipeline.SearchOutput{
				Results: []pipeline.PresentedResult{
					{
						DocumentID: "doc-123",
						ChunkID:    "chunk-456",
						SourceID:   "source-789",
						Title:      "Test Title",
						Snippet:    "Test snippet content",
						Score:      0.87,
						Highlights: []pipeline.Highlight{
							{Field: "content", Text: "highlighted text", Offset: 10},
						},
						Metadata: map[string]any{"custom": "value"},
					},
				},
				TotalCount: 1,
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, capabilitySet)

	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields were mapped correctly
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}

	rankedChunk := result.Results[0]
	if rankedChunk.Score != 0.87 {
		t.Errorf("expected score=0.87, got %f", rankedChunk.Score)
	}
	if rankedChunk.Chunk == nil {
		t.Fatal("expected chunk to be set")
	}
	if rankedChunk.Chunk.ID != "chunk-456" {
		t.Errorf("expected chunk ID='chunk-456', got %s", rankedChunk.Chunk.ID)
	}
	if rankedChunk.Chunk.DocumentID != "doc-123" {
		t.Errorf("expected document ID='doc-123', got %s", rankedChunk.Chunk.DocumentID)
	}
	if rankedChunk.Chunk.SourceID != "source-789" {
		t.Errorf("expected source ID='source-789', got %s", rankedChunk.Chunk.SourceID)
	}
	if rankedChunk.Chunk.Content != "Test snippet content" {
		t.Errorf("expected content='Test snippet content', got %s", rankedChunk.Chunk.Content)
	}
}
