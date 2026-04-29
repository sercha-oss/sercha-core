package search

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const RankerStageID = "ranker"

// DefaultRRFk is the default constant for Reciprocal Rank Fusion.
// Lower values make rank positions more influential (better top-10 separation).
// With k=30, rank 1 vs 5 has ~12% difference vs ~6% with k=60.
const DefaultRRFk = 30


// RankerFactory creates ranker stages.
type RankerFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewRankerFactory creates a new ranker factory.
func NewRankerFactory() *RankerFactory {
	return &RankerFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          RankerStageID,
			Name:        "RRF Ranker",
			Type:        pipeline.StageTypeRanker,
			InputShape:  pipeline.ShapeCandidate,
			OutputShape: pipeline.ShapeRankedResult,
			Cardinality: pipeline.CardinalityManyToMany,
			Version:     "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *RankerFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *RankerFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new ranker stage.
func (f *RankerFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	limit := 20
	if l, ok := config.Parameters["limit"].(float64); ok {
		limit = int(l)
	}

	k := DefaultRRFk
	if kParam, ok := config.Parameters["rrf_k"].(float64); ok && kParam > 0 {
		k = int(kParam)
	}

	return &RankerStage{
		descriptor: f.descriptor,
		limit:      limit,
		rrfK:       k,
	}, nil
}

// Validate validates the stage configuration.
func (f *RankerFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// RankerStage ranks candidates using Reciprocal Rank Fusion (RRF) at document level.
//
// BM25 candidates are already document-level (ChunkID="").
// Vector candidates are chunk-level — multiple chunks from the same document are
// aggregated: the best-scoring chunk represents the document in vector rankings.
//
// RRF then computes: score = Σ 1/(k + rank_i) across source rankings (bm25, vector)
// where rank_i is the document's rank in each source.
type RankerStage struct {
	descriptor pipeline.StageDescriptor
	limit      int
	rrfK       int
}

// Descriptor returns the stage descriptor.
func (s *RankerStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process ranks input candidates using document-level RRF.
func (s *RankerStage) Process(ctx context.Context, input any) (any, error) {
	candidates, ok := input.([]*pipeline.Candidate)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Candidate"}
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	// Group candidates by source (e.g., "bm25", "vector")
	bySource := make(map[string][]*pipeline.Candidate)
	for _, c := range candidates {
		src := c.Source
		if src == "" {
			src = "unknown"
		}
		bySource[src] = append(bySource[src], c)
	}

	// For each source, aggregate to document-level (best candidate per
	// document) and sort by score descending to establish rank order.
	//
	// The previous code applied a SECOND vector-similarity filter here on
	// top of the multi_retriever's distance threshold. That double-filter
	// was a 50% ceiling source on dense corpora — the ranker ended up with
	// 4-7 vector candidates vs ~95 BM25, which left the multi-source RRF
	// math effectively single-source. Drop the secondary filter; rely on
	// the upstream threshold + RRF rank-weighting.
	docBySource := make(map[string][]*pipeline.Candidate) // source -> document-level candidates
	for src, srcCandidates := range bySource {
		docBySource[src] = aggregateToDocLevel(srcCandidates)
	}

	// Per-source post-aggregation counts. Distinguishing "vector returned
	// nothing" from "vector returned 20 unique docs after dedup" is the
	// diagnostic we keep needing.
	sourceSummary := map[string]int{}
	for src, docs := range docBySource {
		sourceSummary[src] = len(docs)
	}
	preAggSummary := map[string]int{}
	for src, cs := range bySource {
		preAggSummary[src] = len(cs)
	}
	slog.Debug("ranker.aggregated by source",
		"phase", "rank_aggregate",
		"source_count", len(docBySource),
		"pre_aggregate_per_source", preAggSummary,
		"post_aggregate_per_source", sourceSummary,
	)

	// Compute RRF scores: for each unique document, sum 1/(k + rank) across sources
	type rrfEntry struct {
		candidate *pipeline.Candidate
		rrfScore  float64
		sources   []string
	}

	merged := make(map[string]*rrfEntry) // keyed by DocumentID

	for source, docCandidates := range docBySource {
		for rank, c := range docCandidates {
			entry, exists := merged[c.DocumentID]
			if !exists {
				entry = &rrfEntry{candidate: c}
				merged[c.DocumentID] = entry
			} else if c.Content != "" && entry.candidate.Content == "" {
				// Prefer candidate with content (vector results have chunk content)
				entry.candidate.Content = c.Content
				entry.candidate.ChunkID = c.ChunkID
			}
			// RRF formula: 1 / (k + rank), rank is 1-based
			entry.rrfScore += 1.0 / float64(s.rrfK+rank+1)
			entry.sources = append(entry.sources, source)
		}
	}

	numSources := len(docBySource)
	results := make([]*pipeline.Candidate, 0, len(merged))

	if numSources == 1 {
		// Single-source: use min-max normalization of original retriever scores.
		// This preserves actual relevance differences (BM25 magnitude or cosine similarity)
		// instead of the fixed staircase that RRF produces with one source group.
		var maxScore, minScore float64
		first := true
		for _, entry := range merged {
			score := entry.candidate.Score
			if first || score > maxScore {
				maxScore = score
			}
			if first || score < minScore {
				minScore = score
			}
			first = false
		}

		scoreRange := maxScore - minScore
		for _, entry := range merged {
			c := entry.candidate
			if scoreRange > 0 {
				c.Score = ((c.Score - minScore) / scoreRange) * 100.0
			} else {
				c.Score = 100.0
			}
			if c.Metadata == nil {
				c.Metadata = make(map[string]any)
			}
			c.Metadata["rrf_sources"] = entry.sources
			c.Metadata["rrf_raw_score"] = entry.rrfScore
			results = append(results, c)
		}
	} else {
		// Multi-source: RRF normalization relative to theoretical maximum.
		// A document at rank 1 in every source scores 100%.
		theoreticalMax := float64(numSources) * (1.0 / float64(s.rrfK+1))
		for _, entry := range merged {
			c := entry.candidate
			if theoreticalMax > 0 {
				c.Score = (entry.rrfScore / theoreticalMax) * 100.0
			} else {
				c.Score = entry.rrfScore
			}
			if c.Metadata == nil {
				c.Metadata = make(map[string]any)
			}
			c.Metadata["rrf_sources"] = entry.sources
			c.Metadata["rrf_raw_score"] = entry.rrfScore
			results = append(results, c)
		}
	}

	// Sort by RRF score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if len(results) > s.limit {
		results = results[:s.limit]
	}

	// Final ranker emit: top-5 (DocumentID, score, sources). The score
	// printed here is the post-normalisation value the UI receives.
	top5 := make([]string, 0, 5)
	n := len(results)
	if n > 5 {
		n = 5
	}
	for i := 0; i < n; i++ {
		c := results[i]
		var srcs []string
		if c.Metadata != nil {
			if v, ok := c.Metadata["rrf_sources"].([]string); ok {
				srcs = v
			}
		}
		top5 = append(top5, c.DocumentID+"@"+formatScore(c.Score)+" "+srcsString(srcs))
	}
	slog.Debug("ranker.final ranking",
		"phase", "rank_final",
		"normalisation_path", normalisationPath(numSources),
		"rrf_k", s.rrfK,
		"output_count", len(results),
		"top5", top5,
	)

	return results, nil
}

// normalisationPath identifies which scoring branch ranker.Process took.
// Single-source uses min-max normalisation of raw retriever scores; the
// multi-source path uses RRF-divided-by-theoretical-max. The choice
// matters for interpreting the final UI score.
func normalisationPath(numSources int) string {
	if numSources == 1 {
		return "single_source_minmax"
	}
	return "multi_source_rrf"
}

// srcsString renders a small []string of source names compactly. Returns
// empty string when nil or empty so log lines stay tidy.
func srcsString(srcs []string) string {
	if len(srcs) == 0 {
		return ""
	}
	return "(" + strings.Join(srcs, ",") + ")"
}

// aggregateToDocLevel groups candidates by DocumentID, keeping the best-scoring
// candidate per document, then sorts by score descending for ranking.
func aggregateToDocLevel(candidates []*pipeline.Candidate) []*pipeline.Candidate {
	bestByDoc := make(map[string]*pipeline.Candidate)
	for _, c := range candidates {
		existing, ok := bestByDoc[c.DocumentID]
		if !ok || c.Score > existing.Score {
			bestByDoc[c.DocumentID] = c
		}
	}

	result := make([]*pipeline.Candidate, 0, len(bestByDoc))
	for _, c := range bestByDoc {
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

// Ensure RankerFactory implements StageFactory.
var _ pipelineport.StageFactory = (*RankerFactory)(nil)

// Ensure RankerStage implements Stage.
var _ pipelineport.Stage = (*RankerStage)(nil)
