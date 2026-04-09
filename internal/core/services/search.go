package services

import (
	"context"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// Ensure searchService implements SearchService
var _ driving.SearchService = (*searchService)(nil)

// searchService implements the SearchService interface
type searchService struct {
	searchEngine    driven.SearchEngine
	documentStore   driven.DocumentStore
	services        *runtime.Services           // Dynamic AI services
	searchExecutor  pipelineport.SearchExecutor // Required pipeline executor
	capabilityStore driven.CapabilityStore      // For fetching capability preferences
	settingsStore   driven.SettingsStore        // For loading team settings
	teamID          string                      // Team ID for settings lookup
}

// NewSearchService creates a new SearchService
// AI services (embedding, LLM) are accessed dynamically via runtime.Services
func NewSearchService(
	searchEngine driven.SearchEngine,
	documentStore driven.DocumentStore,
	services *runtime.Services,
	searchExecutor pipelineport.SearchExecutor, // Required pipeline executor
	capabilityStore driven.CapabilityStore, // For fetching capability preferences
	settingsStore driven.SettingsStore, // For loading team settings
	teamID string, // Team ID for settings lookup
) driving.SearchService {
	// SearchExecutor is now required
	if searchExecutor == nil {
		panic("SearchExecutor is required for SearchService")
	}

	return &searchService{
		searchEngine:    searchEngine,
		documentStore:   documentStore,
		services:        services,
		searchExecutor:  searchExecutor,
		capabilityStore: capabilityStore,
		settingsStore:   settingsStore,
		teamID:          teamID,
	}
}

// Search performs a search across all sources
func (s *searchService) Search(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	start := time.Now()

	// Load settings to apply defaults
	settings, err := s.loadSettings(ctx)
	if err != nil {
		// If settings can't be loaded, use hardcoded defaults
		settings = domain.DefaultSettings(s.teamID)
	}

	// Apply settings-based defaults
	if opts.Limit <= 0 {
		opts.Limit = settings.ResultsPerPage
	}
	// Enforce max limit from settings
	if opts.Limit > settings.MaxResultsPerPage {
		opts.Limit = settings.MaxResultsPerPage
	}

	// Apply default search mode if not specified
	if opts.Mode == "" {
		opts.Mode = settings.DefaultSearchMode
	}

	// Use pipeline executor (required)
	return s.searchWithPipeline(ctx, query, opts, start)
}

// searchWithPipeline performs search using the pipeline executor.
func (s *searchService) searchWithPipeline(
	ctx context.Context,
	query string,
	opts domain.SearchOptions,
	start time.Time,
) (*domain.SearchResult, error) {
	// Build pipeline input
	pipelineInput := &pipeline.SearchInput{
		Query: query,
		Filters: pipeline.SearchFilters{
			Custom: make(map[string]any),
		},
	}

	// Map domain search options to pipeline filters
	if len(opts.SourceIDs) > 0 {
		pipelineInput.Filters.Sources = opts.SourceIDs
	}

	// Build pipeline context
	pipelineContext := &pipeline.SearchContext{
		PipelineID: "default-search",
		Filters:    pipelineInput.Filters,
		Pagination: pipeline.PaginationConfig{
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	}

	// Start with defaults: BM25 enabled, vector disabled
	pipelineContext.Preferences = &pipeline.StagePreferences{
		BM25SearchEnabled:   true,
		VectorSearchEnabled: false,
	}

	// Fetch capability preferences (what's available)
	// Note: teamID should come from context, use "default" for now
	if s.capabilityStore != nil {
		prefs, _ := s.capabilityStore.GetPreferences(ctx, "default")
		if prefs != nil {
			pipelineContext.Preferences.TextIndexingEnabled = prefs.TextIndexingEnabled
			pipelineContext.Preferences.EmbeddingIndexingEnabled = prefs.EmbeddingIndexingEnabled
			pipelineContext.Preferences.BM25SearchEnabled = prefs.BM25SearchEnabled
			pipelineContext.Preferences.VectorSearchEnabled = prefs.VectorSearchEnabled
		}
	}

	// Apply search mode on top of capability availability
	switch opts.Mode {
	case domain.SearchModeTextOnly:
		pipelineContext.Preferences.VectorSearchEnabled = false
	case domain.SearchModeSemanticOnly:
		pipelineContext.Preferences.BM25SearchEnabled = false
	case domain.SearchModeHybrid:
		// Keep both as-is from capability preferences
	}

	// Execute pipeline
	pipelineOutput, err := s.searchExecutor.Execute(ctx, pipelineContext, pipelineInput)
	if err != nil {
		return nil, err
	}

	// Convert pipeline results to domain results, filtering out orphaned documents
	items := make([]*domain.SearchResultItem, 0, len(pipelineOutput.Results))
	for _, result := range pipelineOutput.Results {
		// Look up the document — skip results where the document no longer exists
		doc, err := s.documentStore.Get(ctx, result.DocumentID)
		if err != nil || doc == nil {
			continue
		}

		items = append(items, &domain.SearchResultItem{
			DocumentID: result.DocumentID,
			SourceID:   doc.SourceID,
			Title:      doc.Title,
			Path:       doc.Path,
			MimeType:   doc.MimeType,
			Snippet:    result.Snippet,
			Score:      result.Score,
			IndexedAt:  doc.IndexedAt,
		})
	}

	// Apply pagination (offset/limit) to the full result set
	totalCount := len(items)
	if opts.Offset > 0 {
		if opts.Offset >= len(items) {
			items = nil
		} else {
			items = items[opts.Offset:]
		}
	}
	if opts.Limit > 0 && opts.Limit < len(items) {
		items = items[:opts.Limit]
	}

	return &domain.SearchResult{
		Query:      query,
		Mode:       opts.Mode,
		Results:    items,
		TotalCount: totalCount,
		Took:       time.Since(start),
	}, nil
}


// SearchBySource performs a search within a specific source
func (s *searchService) SearchBySource(ctx context.Context, sourceID string, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	// Add source filter
	opts.SourceIDs = []string{sourceID}
	return s.Search(ctx, query, opts)
}

// loadSettings loads team settings for the search service
func (s *searchService) loadSettings(ctx context.Context) (*domain.Settings, error) {
	if s.settingsStore == nil {
		return domain.DefaultSettings(s.teamID), nil
	}
	return s.settingsStore.GetSettings(ctx, s.teamID)
}

