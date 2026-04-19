package services

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
)

// TestSearchService_AppliesResultsPerPageSetting validates acceptance criteria:
// - Search uses `results_per_page` as default limit when not specified by client
func TestSearchService_AppliesResultsPerPageSetting(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(nil)

	// Create settings store with custom results_per_page
	customSettings := domain.DefaultSettings("team-123")
	customSettings.ResultsPerPage = 15 // Custom value
	settingsStore := &mockSettingsStore{
		settings: customSettings,
	}

	// Create search service with settings store
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			// Verify that the limit from settings is applied
			if sctx.Pagination.Limit != 15 {
				t.Errorf("expected limit 15 from settings, got %d", sctx.Pagination.Limit)
			}
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, settingsStore, "team-123").(*searchService)

	// Search without specifying limit
	_, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode: domain.SearchModeTextOnly,
		// Limit is intentionally not set
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSearchService_AppliesMaxResultsPerPageSetting validates acceptance criteria:
// - Search uses `max_results_per_page` as ceiling for limit
func TestSearchService_AppliesMaxResultsPerPageSetting(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(nil)

	// Create settings store with custom max_results_per_page
	customSettings := domain.DefaultSettings("team-123")
	customSettings.MaxResultsPerPage = 50 // Custom ceiling
	settingsStore := &mockSettingsStore{
		settings: customSettings,
	}

	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			// Verify that the limit is capped at max_results_per_page
			if sctx.Pagination.Limit != 50 {
				t.Errorf("expected limit to be capped at 50, got %d", sctx.Pagination.Limit)
			}
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, settingsStore, "team-123").(*searchService)

	// Search with limit exceeding max_results_per_page
	_, err := svc.Search(context.Background(), "test", domain.SearchOptions{
		Mode:  domain.SearchModeTextOnly,
		Limit: 200, // Exceeds the 50 max
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSearchService_AppliesDefaultSearchMode validates acceptance criteria:
// - Search uses `default_search_mode` when mode not specified by client
func TestSearchService_AppliesDefaultSearchMode(t *testing.T) {
	tests := []struct {
		name         string
		defaultMode  domain.SearchMode
		clientMode   domain.SearchMode
		expectedMode domain.SearchMode
	}{
		{
			name:         "uses default mode when client doesn't specify",
			defaultMode:  domain.SearchModeSemanticOnly,
			clientMode:   "", // Not specified
			expectedMode: domain.SearchModeSemanticOnly,
		},
		{
			name:         "client mode overrides default",
			defaultMode:  domain.SearchModeHybrid,
			clientMode:   domain.SearchModeTextOnly,
			expectedMode: domain.SearchModeTextOnly,
		},
		{
			name:         "default hybrid mode",
			defaultMode:  domain.SearchModeHybrid,
			clientMode:   "",
			expectedMode: domain.SearchModeHybrid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchEngine := mocks.NewMockSearchEngine()
			documentStore := mocks.NewMockDocumentStore()
			runtimeServices := createTestServices(nil)

			customSettings := domain.DefaultSettings("team-123")
			customSettings.DefaultSearchMode = tt.defaultMode
			settingsStore := &mockSettingsStore{
				settings: customSettings,
			}

			executor := &mockSearchExecutor{
				executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
					return &pipeline.SearchOutput{
						Results:    []pipeline.PresentedResult{},
						TotalCount: 0,
					}, nil
				},
			}

			svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, settingsStore, "team-123").(*searchService)

			result, err := svc.Search(context.Background(), "test", domain.SearchOptions{
				Mode: tt.clientMode,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Mode != tt.expectedMode {
				t.Errorf("expected mode %s, got %s", tt.expectedMode, result.Mode)
			}
		})
	}
}

// TestSearchService_FallbackToDefaultSettings validates that search falls back to
// default settings when settings store fails
func TestSearchService_FallbackToDefaultSettings(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(nil)

	// Create settings store that returns nil (simulating not found)
	settingsStore := &mockSettingsStore{
		settings: nil, // Will return ErrNotFound
	}

	limitApplied := 0
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			limitApplied = sctx.Pagination.Limit
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, settingsStore, "team-123").(*searchService)

	// Search should still work with default settings
	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify default limit (20) was applied
	if limitApplied != 20 {
		t.Errorf("expected default limit 20, got %d", limitApplied)
	}

	// Verify default search mode (hybrid) was applied
	if result.Mode != domain.SearchModeHybrid {
		t.Errorf("expected default mode hybrid, got %s", result.Mode)
	}
}

// TestSearchService_SettingsIntegrationEndToEnd validates complete settings integration
func TestSearchService_SettingsIntegrationEndToEnd(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(nil)

	// Create custom settings
	customSettings := domain.DefaultSettings("team-123")
	customSettings.ResultsPerPage = 10
	customSettings.MaxResultsPerPage = 30
	customSettings.DefaultSearchMode = domain.SearchModeTextOnly

	settingsStore := &mockSettingsStore{
		settings: customSettings,
	}

	var capturedLimit int
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedLimit = sctx.Pagination.Limit
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}

	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, settingsStore, "team-123").(*searchService)

	tests := []struct {
		name          string
		options       domain.SearchOptions
		expectedLimit int
		expectedMode  domain.SearchMode
	}{
		{
			name:          "uses default limit when not specified",
			options:       domain.SearchOptions{},
			expectedLimit: 10, // ResultsPerPage
			expectedMode:  domain.SearchModeTextOnly,
		},
		{
			name: "respects client limit within max",
			options: domain.SearchOptions{
				Limit: 25,
			},
			expectedLimit: 25,
			expectedMode:  domain.SearchModeTextOnly,
		},
		{
			name: "caps limit at max_results_per_page",
			options: domain.SearchOptions{
				Limit: 100, // Exceeds 30 max
			},
			expectedLimit: 30, // MaxResultsPerPage
			expectedMode:  domain.SearchModeTextOnly,
		},
		{
			name: "client mode overrides default",
			options: domain.SearchOptions{
				Mode: domain.SearchModeHybrid,
			},
			expectedLimit: 10,
			expectedMode:  domain.SearchModeHybrid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Search(context.Background(), "test", tt.options)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if capturedLimit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, capturedLimit)
			}

			if result.Mode != tt.expectedMode {
				t.Errorf("expected mode %s, got %s", tt.expectedMode, result.Mode)
			}
		})
	}
}

// TestSearchService_NilSettingsStore validates that nil settings store falls back to defaults
func TestSearchService_NilSettingsStore(t *testing.T) {
	searchEngine := mocks.NewMockSearchEngine()
	documentStore := mocks.NewMockDocumentStore()
	runtimeServices := createTestServices(nil)

	var capturedLimit int
	executor := &mockSearchExecutor{
		executeFn: func(ctx context.Context, sctx *pipeline.SearchContext, input *pipeline.SearchInput) (*pipeline.SearchOutput, error) {
			capturedLimit = sctx.Pagination.Limit
			return &pipeline.SearchOutput{
				Results:    []pipeline.PresentedResult{},
				TotalCount: 0,
			}, nil
		},
	}

	// Pass nil settings store
	svc := NewSearchService(searchEngine, documentStore, runtimeServices, executor, nil, nil, "team-123").(*searchService)

	result, err := svc.Search(context.Background(), "test", domain.SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use default settings
	if capturedLimit != 20 {
		t.Errorf("expected default limit 20, got %d", capturedLimit)
	}

	if result.Mode != domain.SearchModeHybrid {
		t.Errorf("expected default mode hybrid, got %s", result.Mode)
	}
}
