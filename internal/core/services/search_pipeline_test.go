package services

import (
	"context"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// createTestServices builds runtime services for search-side tests.
func createTestServices(embeddingService *mocks.MockEmbeddingService) *runtime.Services {
	config := domain.NewRuntimeConfig("postgres")
	services := runtime.NewServices(config)
	if embeddingService != nil {
		services.SetEmbeddingService(embeddingService)
	}
	return services
}

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
				Score:      90.0,
			},
		},
		TotalCount: 1,
		Timing: pipeline.ExecutionTiming{
			TotalMs: 10,
		},
	}, nil
}

// TestSearchService_BoostTerms_FlowToContext verifies that user-supplied
// boost terms reach pipeline stages via SearchContext. The OpenSearch
// adapter still reads them directly off SearchOptions for the standard
// query path; this plumbing is for custom retriever stages that build
// their own queries.
func TestSearchService_BoostTerms_FlowToContext(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(mocks.NewMockEmbeddingService())

	var capturedBoost map[string]float64
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedBoost = sctx.BoostTerms
			return &pipeline.SearchOutput{}, nil
		},
	}
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

	boost := map[string]float64{"kubernetes": 2.0, "helm": 1.5}
	_, err := svc.Search(context.Background(), "q", domain.SearchOptions{
		Limit:      10,
		BoostTerms: boost,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(capturedBoost) != 2 {
		t.Fatalf("captured BoostTerms len = %d, want 2: %v", len(capturedBoost), capturedBoost)
	}
	if capturedBoost["kubernetes"] != 2.0 || capturedBoost["helm"] != 1.5 {
		t.Errorf("boost terms not propagated correctly: %v", capturedBoost)
	}
}

// TestSearchService_PipelineID_RoutingHonoursOptsValue verifies that
// callers can route to a custom registered pipeline by setting
// SearchOptions.PipelineID. Empty falls back to "default-search".
func TestSearchService_PipelineID_RoutingHonoursOptsValue(t *testing.T) {
	cases := []struct {
		name         string
		optsID       string
		wantPipeline string
	}{
		{"empty falls back to default", "", "default-search"},
		{"custom id is honoured", "my-custom-pipeline", "my-custom-pipeline"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			searchEngine := mocks.NewMockSearchEngine()
			documentStore := mocks.NewMockDocumentStore()
			runtimeServices := createTestServices(mocks.NewMockEmbeddingService())

			var capturedPipelineID string
			executor := &mockSearchExecutor{
				executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
					capturedPipelineID = sctx.PipelineID
					return &pipeline.SearchOutput{}, nil
				},
			}
			svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

			_, err := svc.Search(context.Background(), "q", domain.SearchOptions{
				Limit:      10,
				PipelineID: tc.optsID,
			})
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if capturedPipelineID != tc.wantPipeline {
				t.Errorf("pipeline = %q, want %q", capturedPipelineID, tc.wantPipeline)
			}
		})
	}
}

// TestSearchService_WithSearchExecutor tests that SearchService uses pipeline executor when provided
func TestSearchService_WithSearchExecutor(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	embeddingService := mocks.NewMockEmbeddingService()
	runtimeServices := createTestServices(embeddingService)

	// Create mock search executor
	executor := &mockSearchExecutor{}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
						Score:      95.0,
					},
					{
						DocumentID: "doc-2",
						ChunkID:    "chunk-2",
						SourceID:   "source-1",
						Title:      "Result 2",
						Snippet:    "Snippet 2",
						Score:      85.0,
					},
				},
				TotalCount: 2,
				Timing: pipeline.ExecutionTiming{
					TotalMs: 15,
				},
			}, nil
		},
	}
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
	if result.Results[0].Score != 95.0 {
		t.Errorf("expected first result score=95.0, got %f", result.Results[0].Score)
	}
	if result.Results[0].Snippet != "Snippet 1" {
		t.Errorf("expected snippet 'Snippet 1', got %s", result.Results[0].Snippet)
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
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
						Score:      90.0,
					},
				},
				TotalCount: 1,
			}, nil
		},
	}
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
	if result.Results[0].Title != "Full Document Title" {
		t.Errorf("expected enriched document title, got %s", result.Results[0].Title)
	}
	if result.Results[0].Path != "/path/to/doc" {
		t.Errorf("expected enriched document path, got %s", result.Results[0].Path)
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

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
	// Note: Capabilities are now built dynamically by the executor, not passed in constructor
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
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

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
						Score:      90.0,
					},
				},
				TotalCount: 1,
			}, nil
		},
	}
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{ID: "doc-1", SourceID: "source-1", Title: "Doc 1"})

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
						Score:      87.0,
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
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "default")

	// Save document for enrichment
	_ = documentStore.Save(context.Background(), &domain.Document{ID: "doc-123", SourceID: "source-789", Title: "Test Title"})

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

	item := result.Results[0]
	if item.Score != 87.0 {
		t.Errorf("expected score=87.0, got %f", item.Score)
	}
	if item.DocumentID != "doc-123" {
		t.Errorf("expected document ID='doc-123', got %s", item.DocumentID)
	}
	if item.SourceID != "source-789" {
		t.Errorf("expected source ID='source-789', got %s", item.SourceID)
	}
	if item.Snippet != "Test snippet content" {
		t.Errorf("expected snippet='Test snippet content', got %s", item.Snippet)
	}
}
