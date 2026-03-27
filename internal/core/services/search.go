package services

import (
	"context"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
	"github.com/custodia-labs/sercha-core/internal/runtime"
)

// Ensure searchService implements SearchService
var _ driving.SearchService = (*searchService)(nil)

// searchService implements the SearchService interface
type searchService struct {
	searchEngine   driven.SearchEngine
	documentStore  driven.DocumentStore
	services       *runtime.Services // Dynamic AI services
	searchExecutor pipelineport.SearchExecutor // Optional pipeline executor
	capabilitySet  *pipeline.CapabilitySet     // Capabilities for pipeline
}

// NewSearchService creates a new SearchService
// AI services (embedding, LLM) are accessed dynamically via runtime.Services
func NewSearchService(
	searchEngine driven.SearchEngine,
	documentStore driven.DocumentStore,
	services *runtime.Services,
	searchExecutor pipelineport.SearchExecutor, // Optional pipeline executor
	capabilitySet *pipeline.CapabilitySet, // Optional capabilities
) driving.SearchService {
	return &searchService{
		searchEngine:   searchEngine,
		documentStore:  documentStore,
		services:       services,
		searchExecutor: searchExecutor,
		capabilitySet:  capabilitySet,
	}
}

// Search performs a search across all sources
func (s *searchService) Search(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	start := time.Now()

	// Apply defaults
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}

	// Try pipeline executor first
	if s.searchExecutor != nil {
		result, err := s.searchWithPipeline(ctx, query, opts, start)
		if err == nil {
			return result, nil
		}
		// Log error but fall back to legacy silently
		// TODO: Add logging
	}

	// Fallback: Use legacy search engine directly
	return s.searchWithLegacy(ctx, query, opts, start)
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
		PipelineID:   "default-search",
		Capabilities: s.capabilitySet,
		Filters:      pipelineInput.Filters,
		Pagination: pipeline.PaginationConfig{
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	}

	// Execute pipeline
	pipelineOutput, err := s.searchExecutor.Execute(ctx, pipelineContext, pipelineInput)
	if err != nil {
		return nil, err
	}

	// Convert pipeline results to domain results
	rankedChunks := make([]*domain.RankedChunk, 0, len(pipelineOutput.Results))
	for _, result := range pipelineOutput.Results {
		// Map pipeline.PresentedResult to domain.RankedChunk
		rankedChunk := &domain.RankedChunk{
			Chunk: &domain.Chunk{
				ID:         result.ChunkID,
				DocumentID: result.DocumentID,
				SourceID:   result.SourceID,
				Content:    result.Snippet,
			},
			Score: result.Score,
		}

		// Enrich with document if needed
		if rankedChunk.Document == nil {
			doc, _ := s.documentStore.Get(ctx, result.DocumentID)
			rankedChunk.Document = doc
		}

		rankedChunks = append(rankedChunks, rankedChunk)
	}

	return &domain.SearchResult{
		Query:      query,
		Mode:       opts.Mode,
		Results:    rankedChunks,
		TotalCount: int(pipelineOutput.TotalCount),
		Took:       time.Since(start),
	}, nil
}

// searchWithLegacy performs search using the legacy search engine.
func (s *searchService) searchWithLegacy(
	ctx context.Context,
	query string,
	opts domain.SearchOptions,
	start time.Time,
) (*domain.SearchResult, error) {
	// Determine effective search mode based on what's available NOW
	opts.Mode = s.effectiveMode(opts.Mode)

	// Get embedding service dynamically (may have been configured at runtime)
	embeddingService := s.services.EmbeddingService()

	// Generate query embedding for semantic search
	var queryEmbedding []float32
	if opts.Mode.RequiresEmbedding() {
		if embeddingService != nil {
			embedding, err := embeddingService.EmbedQuery(ctx, query)
			if err != nil {
				// Fall back to text-only if embedding fails
				opts.Mode = domain.SearchModeTextOnly
			} else {
				queryEmbedding = embedding
			}
		} else {
			// Embedding required but not available - degrade
			opts.Mode = domain.SearchModeTextOnly
		}
	}

	// Perform search
	rankedChunks, totalCount, err := s.searchEngine.Search(ctx, query, queryEmbedding, opts)
	if err != nil {
		return nil, err
	}

	// Enrich with document data
	for _, rc := range rankedChunks {
		if rc.Document == nil && rc.Chunk != nil {
			doc, _ := s.documentStore.Get(ctx, rc.Chunk.DocumentID)
			rc.Document = doc
		}
	}

	return &domain.SearchResult{
		Query:      query,
		Mode:       opts.Mode,
		Results:    rankedChunks,
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

// Suggest provides search suggestions/autocomplete
func (s *searchService) Suggest(_ context.Context, _ string, _ int) ([]domain.SearchSuggestion, error) {
	// TODO: Implement autocomplete/suggestions
	// This could use:
	// 1. Recent search history
	// 2. Popular terms from indexed content
	// 3. Prefix matching on document titles
	return []domain.SearchSuggestion{}, nil
}

// effectiveMode determines the best search mode based on requested mode and available services
func (s *searchService) effectiveMode(requested domain.SearchMode) domain.SearchMode {
	// Default to hybrid if not specified
	if requested == "" {
		requested = s.services.Config().EffectiveSearchMode()
	}

	config := s.services.Config()

	// Validate requested mode is possible with current capabilities
	switch requested {
	case domain.SearchModeSemanticOnly:
		if !config.CanDoSemanticSearch() {
			return domain.SearchModeTextOnly
		}
	case domain.SearchModeHybrid:
		if !config.CanDoSemanticSearch() {
			return domain.SearchModeTextOnly
		}
	}

	return requested
}
