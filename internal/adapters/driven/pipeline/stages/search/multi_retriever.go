package search

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	MultiRetrieverStageID = "multi-retriever"
	DefaultTopK           = 100
	DefaultRRFK           = 60
	// DefaultVectorDistanceThreshold is a loose safety floor only — a
	// distance ≤ 0.7 corresponds to cosine similarity ≥ 0.3, which keeps
	// genuinely off-topic chunks out while letting RRF rank-weighting
	// handle relevance ordering. Tightening this further (the previous
	// 0.55 value) routinely dropped 96+ of 100 candidates on dense
	// corpora, starving the ranker of vector signal entirely.
	DefaultVectorDistanceThreshold = 0.7
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

	// Per-variant counts plus the post-merge top-3 — useful for spotting
	// "variant 0 returned 20 docs, variants 1-2 returned 0" patterns or
	// "merge stripped 80% of unique docs because every variant returned
	// the same set".
	variantCounts := make([]int, len(allResults))
	for i, r := range allResults {
		variantCounts[i] = len(r)
	}
	slog.Debug("search.merge_variants completed",
		"phase", "merge_variants",
		"variant_count", len(queries),
		"variant_candidate_counts", variantCounts,
		"merged_count", len(merged),
		"top3", topNSummary(merged, 3),
	)

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
	//
	// When the user submits only a quoted phrase the parser leaves Terms
	// empty. The OpenSearch adapter's must clause is a multi_match against
	// queryStr, and an empty queryStr makes the must clause match nothing —
	// the phrase clauses alone in opts.Phrases aren't enough because they
	// also live in must. Fall back to joined phrase content so the must
	// clause has tokens to score on; the match_phrase clauses still apply
	// the strict-contiguity check on top.
	queryStr := strings.Join(q.Terms, " ")
	if queryStr == "" {
		queryStr = strings.Join(q.Phrases, " ")
	}
	if queryStr == "" {
		queryStr = q.Original
	}

	// BoostTerms must be copied here from the parsed query — the OpenSearch
	// adapter consumes them off SearchOptions to build should-clauses, but
	// the pipeline executor doesn't propagate them automatically. Without
	// this line user-supplied boost terms silently no-op on every search.
	opts := domain.SearchOptions{
		Limit:            s.topK,
		Mode:             domain.SearchModeTextOnly,
		SourceIDs:        q.SearchFilters.Sources,
		DocumentIDFilter: q.SearchFilters.DocumentIDFilter,
		Phrases:          q.Phrases,
		BoostTerms:       q.BoostTerms,
	}

	bm25Results, _, err := s.searchEngine.SearchDocuments(ctx, queryStr, opts)
	if err != nil {
		slog.Warn("search.bm25 failed",
			"phase", "retrieve_bm25",
			"query", queryStr,
			"phrases", q.Phrases,
			"error", err,
		)
		return nil, err
	}

	bm25Cands := convertDocResultsToCandidates(bm25Results, "bm25")
	slog.Debug("search.bm25 returned candidates",
		"phase", "retrieve_bm25",
		"query", queryStr,
		"phrase_count", len(q.Phrases),
		"candidate_count", len(bm25Cands),
		"top3", topNSummary(bm25Cands, 3),
	)
	candidates = append(candidates, bm25Cands...)

	// Vector search (optional - only if both vectorIndex and embedder are available
	// and the admin pref VectorSearchEnabled hasn't disabled it via stage config)
	switch {
	case s.disableVector:
		slog.Debug("search.vector skipped",
			"phase", "retrieve_vector",
			"reason", "disable_vector_set",
		)
	case s.vectorIndex == nil:
		slog.Debug("search.vector skipped",
			"phase", "retrieve_vector",
			"reason", "vector_index_unavailable",
		)
	case s.embedder == nil:
		slog.Debug("search.vector skipped",
			"phase", "retrieve_vector",
			"reason", "embedder_unavailable",
		)
	default:
		queryEmbedding, err := s.embedder.EmbedQuery(ctx, q.Original)
		if err != nil {
			slog.Warn("search.vector embed_query failed",
				"phase", "retrieve_vector",
				"error", err,
			)
			return candidates, nil
		}

		vectorResults, err := s.vectorIndex.SearchWithContent(ctx, queryEmbedding, s.topK, q.SearchFilters.Sources, q.SearchFilters.DocumentIDFilter)
		if err != nil {
			slog.Warn("search.vector lookup failed",
				"phase", "retrieve_vector",
				"error", err,
			)
			return candidates, nil
		}

		// Filter by distance threshold - only include semantically close results
		filteredResults := make([]driven.VectorSearchResult, 0)
		for _, vr := range vectorResults {
			if vr.Distance <= s.vectorDistanceThreshold {
				filteredResults = append(filteredResults, vr)
			}
		}

		vectorChunkCands := convertVectorResultsToCandidates(filteredResults, "vector")

		// Aggregate chunk-level vector candidates to doc-level (best chunk
		// per document). pgvector retrieves at chunk granularity, but
		// downstream BM25 candidates are doc-level — leaving them at chunk
		// granularity means the variant-merger and ranker double-count
		// vector contributions for any doc that has multiple matching
		// chunks. Aggregating here makes BM25 and vector compete at the
		// same unit of retrieval.
		vectorCands := bestChunkPerDoc(vectorChunkCands)

		slog.Debug("search.vector returned candidates",
			"phase", "retrieve_vector",
			"raw_count", len(vectorResults),
			"after_threshold_count", len(filteredResults),
			"after_doc_aggregate_count", len(vectorCands),
			"distance_threshold", s.vectorDistanceThreshold,
			"top3", topNSummary(vectorCands, 3),
		)
		candidates = append(candidates, vectorCands...)
	}

	return candidates, nil
}

// topNSummary returns up to n compact summaries (DocumentID + score) for
// quick log-eyeball diagnostics. Returned as a slice of strings so it
// renders nicely in slog text/JSON output.
func topNSummary(cands []*pipeline.Candidate, n int) []string {
	if len(cands) < n {
		n = len(cands)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		c := cands[i]
		out = append(out, fmtSummary(c))
	}
	return out
}

// fmtSummary builds a single-candidate summary line.
func fmtSummary(c *pipeline.Candidate) string {
	if c == nil {
		return "<nil>"
	}
	return c.DocumentID + "@" + formatScore(c.Score)
}

// formatScore truncates a score to 4 significant digits for log readability.
func formatScore(f float64) string {
	return strconv.FormatFloat(f, 'g', 4, 64)
}

// bestChunkPerDoc collapses chunk-level vector candidates to one
// candidate per DocumentID, keeping the highest-scoring chunk. The
// returned slice preserves descending-score order, so downstream
// rank-based fusions see the same ordering they did before
// aggregation.
//
// Vector retrieval returns chunks because pgvector indexes chunk
// embeddings; BM25 retrieves at document level. Aggregating here puts
// both retrievers on the same footing for the variant-merger and the
// final ranker — without it, a doc with five matching chunks
// contributes five RRF terms while a doc with one matching chunk
// contributes one, even though both are "this document was a good
// match".
func bestChunkPerDoc(chunks []*pipeline.Candidate) []*pipeline.Candidate {
	if len(chunks) == 0 {
		return chunks
	}
	bestByDoc := make(map[string]*pipeline.Candidate, len(chunks))
	order := make([]string, 0, len(chunks))
	for _, c := range chunks {
		existing, ok := bestByDoc[c.DocumentID]
		if !ok {
			bestByDoc[c.DocumentID] = c
			order = append(order, c.DocumentID)
			continue
		}
		if c.Score > existing.Score {
			bestByDoc[c.DocumentID] = c
		}
	}
	out := make([]*pipeline.Candidate, 0, len(bestByDoc))
	for _, id := range order {
		out = append(out, bestByDoc[id])
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
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

	// Group candidates by (DocumentID, Source) for deduplication.
	//
	// The previous key was DocumentID alone, which collapsed BM25 and
	// vector hits for the same doc into a single candidate whose Source
	// happened to be whichever retriever appended first (always BM25 in
	// runSearch). The downstream RankerStage then re-buckets by Source
	// to compute multi-source RRF — so collapsing here meant docs found
	// in BOTH retrievers only contributed to one bucket. The ranker's
	// theoretical-max formula caps such single-source matches at 50%,
	// which is the cap pattern we kept seeing in production traces.
	//
	// Keying on (DocumentID, Source) preserves both retriever signals
	// up to the ranker, which is the stage semantically aware of source
	// fusion.
	type rrfKey struct {
		docID  string
		source string
	}
	type rrfEntry struct {
		candidate *pipeline.Candidate
		score     float64
		variants  []int // Which query variants found this doc
	}

	docScores := make(map[rrfKey]*rrfEntry)

	for variantIdx, variantCandidates := range results {
		if variantCandidates == nil {
			continue
		}

		// Rank each source independently within this variant.
		// runSearch returns BM25 results followed by vector results
		// concatenated; using the position in the combined slice as
		// the RRF rank starves the second source — its rank starts
		// where the first source's count ends, putting every vector
		// candidate at rank >= 100 even when it's the top match in
		// its own retriever. Per-source rank assignment puts both
		// retrievers on the same rank scale.
		sourceRank := make(map[string]int)
		for _, candidate := range variantCandidates {
			rank := sourceRank[candidate.Source]
			sourceRank[candidate.Source] = rank + 1

			key := rrfKey{docID: candidate.DocumentID, source: candidate.Source}

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
