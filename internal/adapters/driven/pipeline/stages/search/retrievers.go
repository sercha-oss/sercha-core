package search

import (
	"context"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	BM25RetrieverStageID   = "bm25-retriever"
	VectorRetrieverStageID = "vector-retriever"
	HybridRetrieverStageID = "hybrid-retriever"
	DefaultTopK            = 100
)

// --- BM25 Retriever ---

// BM25RetrieverFactory creates BM25 retriever stages.
type BM25RetrieverFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewBM25RetrieverFactory creates a new BM25 retriever factory.
func NewBM25RetrieverFactory() *BM25RetrieverFactory {
	return &BM25RetrieverFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          BM25RetrieverStageID,
			Name:        "BM25 Retriever",
			Type:        pipeline.StageTypeRetriever,
			InputShape:  pipeline.ShapeParsedQuery,
			OutputShape: pipeline.ShapeCandidate,
			Cardinality: pipeline.CardinalityOneToMany,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilitySearchEngine, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

func (f *BM25RetrieverFactory) StageID() string                            { return f.descriptor.ID }
func (f *BM25RetrieverFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *BM25RetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *BM25RetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	inst, ok := capabilities.Get(pipeline.CapabilitySearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "search_engine capability not available"}
	}

	searchEngine, ok := inst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid search_engine instance type"}
	}

	topK := DefaultTopK
	if k, ok := config.Parameters["top_k"].(float64); ok {
		topK = int(k)
	}

	return &BM25RetrieverStage{
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
		topK:         topK,
	}, nil
}

// BM25RetrieverStage retrieves document-level candidates using BM25 text search.
type BM25RetrieverStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
	topK         int
}

func (s *BM25RetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *BM25RetrieverStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	queryStr := strings.Join(parsed.Terms, " ")
	if len(parsed.Phrases) > 0 {
		queryStr += " " + strings.Join(parsed.Phrases, " ")
	}

	opts := domain.SearchOptions{
		Limit:     s.topK,
		Mode:      domain.SearchModeTextOnly,
		SourceIDs: parsed.SearchFilters.Sources,
	}

	// Use SearchDocuments for document-level BM25 results
	results, _, err := s.searchEngine.SearchDocuments(ctx, queryStr, opts)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "search failed", Err: err}
	}

	candidates := convertDocResultsToCandidates(results, "bm25")
	return candidates, nil
}

// --- Vector Retriever ---

// VectorRetrieverFactory creates vector retriever stages.
type VectorRetrieverFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewVectorRetrieverFactory creates a new vector retriever factory.
func NewVectorRetrieverFactory() *VectorRetrieverFactory {
	return &VectorRetrieverFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          VectorRetrieverStageID,
			Name:        "Vector Retriever",
			Type:        pipeline.StageTypeRetriever,
			InputShape:  pipeline.ShapeParsedQuery,
			OutputShape: pipeline.ShapeCandidate,
			Cardinality: pipeline.CardinalityOneToMany,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

func (f *VectorRetrieverFactory) StageID() string                            { return f.descriptor.ID }
func (f *VectorRetrieverFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *VectorRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *VectorRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	vectorInst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}
	vectorIndex, ok := vectorInst.Instance.(driven.VectorIndex)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid vector_store instance type"}
	}

	embedInst, ok := capabilities.Get(pipeline.CapabilityEmbedder)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "embedder capability not available"}
	}
	embedder, ok := embedInst.Instance.(driven.EmbeddingService)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid embedder instance type"}
	}

	topK := DefaultTopK
	if k, ok := config.Parameters["top_k"].(float64); ok {
		topK = int(k)
	}

	return &VectorRetrieverStage{
		descriptor:  f.descriptor,
		vectorIndex: vectorIndex,
		embedder:    embedder,
		topK:        topK,
	}, nil
}

// VectorRetrieverStage retrieves chunk-level candidates using vector similarity via pgvector.
type VectorRetrieverStage struct {
	descriptor  pipeline.StageDescriptor
	vectorIndex driven.VectorIndex
	embedder    driven.EmbeddingService
	topK        int
}

func (s *VectorRetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *VectorRetrieverStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	// Generate query embedding
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, parsed.Original)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "embedding failed", Err: err}
	}

	// Search pgvector for similar chunks (returns content alongside)
	results, err := s.vectorIndex.SearchWithContent(ctx, queryEmbedding, s.topK, parsed.SearchFilters.Sources)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "vector search failed", Err: err}
	}

	candidates := convertVectorResultsToCandidates(results, "vector")
	return candidates, nil
}

// --- Hybrid Retriever ---

// HybridRetrieverFactory creates hybrid retriever stages.
type HybridRetrieverFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewHybridRetrieverFactory creates a new hybrid retriever factory.
func NewHybridRetrieverFactory() *HybridRetrieverFactory {
	return &HybridRetrieverFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          HybridRetrieverStageID,
			Name:        "Hybrid Retriever",
			Type:        pipeline.StageTypeRetriever,
			InputShape:  pipeline.ShapeParsedQuery,
			OutputShape: pipeline.ShapeCandidate,
			Cardinality: pipeline.CardinalityOneToMany,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilitySearchEngine, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

func (f *HybridRetrieverFactory) StageID() string                            { return f.descriptor.ID }
func (f *HybridRetrieverFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *HybridRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *HybridRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	searchInst, ok := capabilities.Get(pipeline.CapabilitySearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "search_engine capability not available"}
	}
	searchEngine, ok := searchInst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid search_engine instance type"}
	}

	vectorInst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}
	vectorIndex, ok := vectorInst.Instance.(driven.VectorIndex)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid vector_store instance type"}
	}

	embedInst, ok := capabilities.Get(pipeline.CapabilityEmbedder)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "embedder capability not available"}
	}
	embedder, ok := embedInst.Instance.(driven.EmbeddingService)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid embedder instance type"}
	}

	topK := DefaultTopK
	if k, ok := config.Parameters["top_k"].(float64); ok {
		topK = int(k)
	}

	return &HybridRetrieverStage{
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
		vectorIndex:  vectorIndex,
		embedder:     embedder,
		topK:         topK,
	}, nil
}

// HybridRetrieverStage retrieves candidates using both BM25 (document-level) and vector (chunk-level).
// BM25 results come from OpenSearch (full documents), vector results come from pgvector (chunks).
// The ranker stage fuses both at document level using RRF.
type HybridRetrieverStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
	vectorIndex  driven.VectorIndex
	embedder     driven.EmbeddingService
	topK         int
}

func (s *HybridRetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *HybridRetrieverStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	queryStr := parsed.Original

	// BM25 search: document-level results from OpenSearch
	bm25Opts := domain.SearchOptions{
		Limit:     s.topK,
		Mode:      domain.SearchModeTextOnly,
		SourceIDs: parsed.SearchFilters.Sources,
	}
	bm25Results, _, err := s.searchEngine.SearchDocuments(ctx, queryStr, bm25Opts)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "BM25 search failed", Err: err}
	}

	// Vector search: chunk-level results from pgvector
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, parsed.Original)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "embedding failed", Err: err}
	}

	vectorResults, err := s.vectorIndex.SearchWithContent(ctx, queryEmbedding, s.topK, parsed.SearchFilters.Sources)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "vector search failed", Err: err}
	}

	// Tag each set with its source for RRF fusion
	bm25Candidates := convertDocResultsToCandidates(bm25Results, "bm25")
	vectorCandidates := convertVectorResultsToCandidates(vectorResults, "vector")

	// Merge both sets — the ranker will handle document-level dedup and fusion
	candidates := append(bm25Candidates, vectorCandidates...)
	return candidates, nil
}

// --- Conversion helpers ---

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
	_ pipelineport.StageFactory = (*BM25RetrieverFactory)(nil)
	_ pipelineport.Stage        = (*BM25RetrieverStage)(nil)
	_ pipelineport.StageFactory = (*VectorRetrieverFactory)(nil)
	_ pipelineport.Stage        = (*VectorRetrieverStage)(nil)
	_ pipelineport.StageFactory = (*HybridRetrieverFactory)(nil)
	_ pipelineport.Stage        = (*HybridRetrieverStage)(nil)
)
