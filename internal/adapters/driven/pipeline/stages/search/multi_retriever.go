package search

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	MultiRetrieverStageID          = "multi-retriever"
	DefaultTopK                    = 100
	DefaultRRFK                    = 60
	DefaultVectorDistanceThreshold = 0.55 // Only include vector results closer than this
)

// MultiRetrieverFactory creates multi-query retriever stages.
type MultiRetrieverFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewMultiRetrieverFactory creates a new multi-query retriever factory.
func NewMultiRetrieverFactory() *MultiRetrieverFactory {
	return &MultiRetrieverFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          MultiRetrieverStageID,
			Name:        "Multi-Query Retriever",
			Type:        pipeline.StageTypeRetriever,
			InputShape:  pipeline.ShapeQuerySet,
			OutputShape: pipeline.ShapeCandidate,
			Cardinality: pipeline.CardinalityManyToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilitySearchEngine, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityOptional},
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
			},
			Version: "1.0.0",
		},
	}
}

func (f *MultiRetrieverFactory) StageID() string                            { return f.descriptor.ID }
func (f *MultiRetrieverFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *MultiRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *MultiRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	// SearchEngine is required
	searchInst, ok := capabilities.Get(pipeline.CapabilitySearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "search_engine capability not available"}
	}
	searchEngine, ok := searchInst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid search_engine instance type"}
	}

	// VectorStore and Embedder are optional
	var vectorIndex driven.VectorIndex
	var embedder driven.EmbeddingService

	if vectorInst, ok := capabilities.Get(pipeline.CapabilityVectorStore); ok {
		if vi, ok := vectorInst.Instance.(driven.VectorIndex); ok {
			vectorIndex = vi
		}
	}

	if embedInst, ok := capabilities.Get(pipeline.CapabilityEmbedder); ok {
		if emb, ok := embedInst.Instance.(driven.EmbeddingService); ok {
			embedder = emb
		}
	}

	topK := DefaultTopK
	if k, ok := config.Parameters["top_k"].(float64); ok {
		topK = int(k)
	}

	rrfK := DefaultRRFK
	if k, ok := config.Parameters["rrf_k"].(float64); ok {
		rrfK = int(k)
	}

	vectorDistanceThreshold := DefaultVectorDistanceThreshold
	if t, ok := config.Parameters["vector_distance_threshold"].(float64); ok {
		vectorDistanceThreshold = t
	}

	disableVector, _ := config.Parameters["disable_vector"].(bool)

	return &MultiRetrieverStage{
		descriptor:              f.descriptor,
		searchEngine:            searchEngine,
		vectorIndex:             vectorIndex,
		embedder:                embedder,
		topK:                    topK,
		rrfK:                    rrfK,
		vectorDistanceThreshold: vectorDistanceThreshold,
		disableVector:           disableVector,
	}, nil
}

// MultiRetrieverStage retrieves candidates for multiple query variants in parallel
// and merges them using weighted Reciprocal Rank Fusion (RRF).
type MultiRetrieverStage struct {
	descriptor              pipeline.StageDescriptor
	searchEngine            driven.SearchEngine
	vectorIndex             driven.VectorIndex
	embedder                driven.EmbeddingService
	topK                    int
	rrfK                    int
	vectorDistanceThreshold float64
	disableVector           bool
}

func (s *MultiRetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *MultiRetrieverStage) Process(ctx context.Context, input any) (any, error) {
	queries, ok := input.([]*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.ParsedQuery"}
	}

	if len(queries) == 0 {
		return []*pipeline.Candidate{}, nil
	}

	// Run all queries in parallel
	type queryResult struct {
		index      int
		candidates []*pipeline.Candidate
		err        error
	}

	resultsCh := make(chan queryResult, len(queries))
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query *pipeline.ParsedQuery) {
			defer wg.Done()
			candidates, err := s.runSearch(ctx, query)
			resultsCh <- queryResult{index: idx, candidates: candidates, err: err}
		}(i, q)
	}

	// Wait for all searches to complete
	wg.Wait()
	close(resultsCh)

	// Collect results
	allResults := make([][]*pipeline.Candidate, len(queries))
	for result := range resultsCh {
		if result.err != nil {
			continue
		}
		allResults[result.index] = result.candidates
	}

	// Merge results using weighted RRF
	merged := s.mergeWithRRF(allResults, queries)

	return merged, nil
}

// runSearch executes both BM25 and optionally vector search for a single query variant.
func (s *MultiRetrieverStage) runSearch(ctx context.Context, q *pipeline.ParsedQuery) ([]*pipeline.Candidate, error) {
	var candidates []*pipeline.Candidate

	// Pass terms as the loose-match query string, phrases via SearchOptions so
	// the OpenSearch adapter can build match_phrase clauses for them. Using
	// q.Original would leak the literal `"` characters into the analyser,
	// which strips them as punctuation and silently degrades the phrase to
	// two unrelated tokens.
	queryStr := strings.Join(q.Terms, " ")
	if queryStr == "" && len(q.Phrases) == 0 {
		queryStr = q.Original
	}

	opts := domain.SearchOptions{
		Limit:            s.topK,
		Mode:             domain.SearchModeTextOnly,
		SourceIDs:        q.SearchFilters.Sources,
		DocumentIDFilter: q.SearchFilters.DocumentIDFilter,
		Phrases:          q.Phrases,
	}

	bm25Results, _, err := s.searchEngine.SearchDocuments(ctx, queryStr, opts)
	if err != nil {
		return nil, err
	}

	candidates = append(candidates, convertDocResultsToCandidates(bm25Results, "bm25")...)

	// Vector search (optional - only if both vectorIndex and embedder are available
	// and the admin pref VectorSearchEnabled hasn't disabled it via stage config)
	if !s.disableVector && s.vectorIndex != nil && s.embedder != nil {
		queryEmbedding, err := s.embedder.EmbedQuery(ctx, q.Original)
		if err != nil {
			return candidates, nil
		}

		vectorResults, err := s.vectorIndex.SearchWithContent(ctx, queryEmbedding, s.topK, q.SearchFilters.Sources, q.SearchFilters.DocumentIDFilter)
		if err != nil {
			return candidates, nil
		}

		// Filter by distance threshold - only include semantically close results
		filteredResults := make([]driven.VectorSearchResult, 0)
		for _, vr := range vectorResults {
			if vr.Distance <= s.vectorDistanceThreshold {
				filteredResults = append(filteredResults, vr)
			}
		}

		candidates = append(candidates, convertVectorResultsToCandidates(filteredResults, "vector")...)
	}

	return candidates, nil
}

// mergeWithRRF merges results from multiple query variants using weighted Reciprocal Rank Fusion.
// Original query gets weight 1.0, variants get weight 0.8.
func (s *MultiRetrieverStage) mergeWithRRF(results [][]*pipeline.Candidate, queries []*pipeline.ParsedQuery) []*pipeline.Candidate {
	// Define weights: original query gets 1.0, variants get 0.8
	weights := make([]float64, len(queries))
	if len(weights) > 0 {
		weights[0] = 1.0 // Original query
	}
	for i := 1; i < len(weights); i++ {
		weights[i] = 0.8 // Variants
	}

	// Group candidates by DocumentID for deduplication
	type rrfEntry struct {
		candidate *pipeline.Candidate
		score     float64
		variants  []int // Which query variants found this doc
	}

	docScores := make(map[string]*rrfEntry)

	for variantIdx, variantCandidates := range results {
		if variantCandidates == nil {
			continue
		}

		for rank, candidate := range variantCandidates {
			// Use DocumentID as the deduplication key
			key := candidate.DocumentID

			entry, exists := docScores[key]
			if !exists {
				// Make a copy of the candidate for the merged result
				candidateCopy := *candidate
				entry = &rrfEntry{
					candidate: &candidateCopy,
					score:     0,
					variants:  []int{},
				}
				docScores[key] = entry
			}

			// Weighted RRF: weight * 1/(k + rank + 1)
			rrfScore := weights[variantIdx] * (1.0 / float64(s.rrfK+rank+1))
			entry.score += rrfScore
			entry.variants = append(entry.variants, variantIdx)
		}
	}

	// Convert map to slice and sort by RRF score
	merged := make([]*pipeline.Candidate, 0, len(docScores))
	for _, entry := range docScores {
		// Update the score to the RRF score
		entry.candidate.Score = entry.score

		// Tag with variant info in metadata
		if entry.candidate.Metadata == nil {
			entry.candidate.Metadata = make(map[string]any)
		}
		entry.candidate.Metadata["query_variants"] = entry.variants
		entry.candidate.Metadata["rrf_score"] = entry.score

		merged = append(merged, entry.candidate)
	}

	// Sort by RRF score descending
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	// Limit to topK
	if len(merged) > s.topK {
		merged = merged[:s.topK]
	}

	return merged
}

// convertDocResultsToCandidates converts document-level BM25 results to pipeline candidates.
func convertDocResultsToCandidates(results []driven.DocumentResult, source string) []*pipeline.Candidate {
	candidates := make([]*pipeline.Candidate, len(results))
	for i, r := range results {
		candidates[i] = &pipeline.Candidate{
			DocumentID: r.DocumentID,
			ChunkID:    "", // Document-level result, no chunk
			SourceID:   r.SourceID,
			Content:    r.Content,
			Score:      r.Score,
			Source:     source,
			Metadata:   map[string]any{"title": r.Title},
		}
	}
	return candidates
}

// convertVectorResultsToCandidates converts chunk-level vector results to pipeline candidates.
func convertVectorResultsToCandidates(results []driven.VectorSearchResult, source string) []*pipeline.Candidate {
	candidates := make([]*pipeline.Candidate, len(results))
	for i, r := range results {
		// Convert distance to similarity score (1 - cosine_distance for cosine)
		score := 1.0 - r.Distance
		if score < 0 {
			score = 0
		}
		candidates[i] = &pipeline.Candidate{
			DocumentID: r.DocumentID,
			ChunkID:    r.ChunkID,
			SourceID:   "", // pgvector doesn't store source_id; ranker/presenter can resolve via DocumentID
			Content:    r.Content,
			Score:      score,
			Source:     source,
			Metadata:   make(map[string]any),
		}
	}
	return candidates
}

// Interface assertions
var (
	_ pipelineport.StageFactory = (*MultiRetrieverFactory)(nil)
	_ pipelineport.Stage        = (*MultiRetrieverStage)(nil)
)
