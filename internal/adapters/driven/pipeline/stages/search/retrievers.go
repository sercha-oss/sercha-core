package search

import (
	"context"
	"strings"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	BM25RetrieverStageID   = "bm25-retriever"
	VectorRetrieverStageID = "vector-retriever"
	HybridRetrieverStageID = "hybrid-retriever"
	DefaultTopK            = 100
)

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
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

func (f *BM25RetrieverFactory) StageID() string                        { return f.descriptor.ID }
func (f *BM25RetrieverFactory) Descriptor() pipeline.StageDescriptor   { return f.descriptor }
func (f *BM25RetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *BM25RetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	inst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}

	searchEngine, ok := inst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid vector_store instance type"}
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

// BM25RetrieverStage retrieves candidates using BM25.
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

	// Build query string from terms and phrases
	queryStr := strings.Join(parsed.Terms, " ")
	if len(parsed.Phrases) > 0 {
		queryStr += " " + strings.Join(parsed.Phrases, " ")
	}

	opts := domain.SearchOptions{
		Limit: s.topK,
		Mode:  domain.SearchModeTextOnly,
	}

	results, _, err := s.searchEngine.Search(ctx, queryStr, nil, opts)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "search failed", Err: err}
	}

	candidates := convertToCandidates(results, "bm25")
	return candidates, nil
}

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

func (f *VectorRetrieverFactory) StageID() string                        { return f.descriptor.ID }
func (f *VectorRetrieverFactory) Descriptor() pipeline.StageDescriptor   { return f.descriptor }
func (f *VectorRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *VectorRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	searchInst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}
	searchEngine, ok := searchInst.Instance.(driven.SearchEngine)
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
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
		embedder:     embedder,
		topK:         topK,
	}, nil
}

// VectorRetrieverStage retrieves candidates using vector similarity.
type VectorRetrieverStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
	embedder     driven.EmbeddingService
	topK         int
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

	opts := domain.SearchOptions{
		Limit: s.topK,
		Mode:  domain.SearchModeSemanticOnly,
	}

	results, _, err := s.searchEngine.Search(ctx, parsed.Original, queryEmbedding, opts)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "search failed", Err: err}
	}

	candidates := convertToCandidates(results, "vector")
	return candidates, nil
}

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
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

func (f *HybridRetrieverFactory) StageID() string                        { return f.descriptor.ID }
func (f *HybridRetrieverFactory) Descriptor() pipeline.StageDescriptor   { return f.descriptor }
func (f *HybridRetrieverFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *HybridRetrieverFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	searchInst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}
	searchEngine, ok := searchInst.Instance.(driven.SearchEngine)
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

	alpha := 0.5
	if a, ok := config.Parameters["alpha"].(float64); ok {
		alpha = a
	}

	return &HybridRetrieverStage{
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
		embedder:     embedder,
		topK:         topK,
		alpha:        alpha,
	}, nil
}

// HybridRetrieverStage retrieves candidates using hybrid search.
type HybridRetrieverStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
	embedder     driven.EmbeddingService
	topK         int
	alpha        float64 // Weight for semantic vs BM25 (0-1)
}

func (s *HybridRetrieverStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *HybridRetrieverStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	// Generate query embedding
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, parsed.Original)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "embedding failed", Err: err}
	}

	opts := domain.SearchOptions{
		Limit: s.topK,
		Mode:  domain.SearchModeHybrid,
	}

	results, _, err := s.searchEngine.Search(ctx, parsed.Original, queryEmbedding, opts)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "search failed", Err: err}
	}

	candidates := convertToCandidates(results, "hybrid")
	return candidates, nil
}

// convertToCandidates converts ranked chunks to pipeline candidates.
func convertToCandidates(results []*domain.RankedChunk, source string) []*pipeline.Candidate {
	candidates := make([]*pipeline.Candidate, len(results))
	for i, r := range results {
		var documentID, chunkID, sourceID, content string
		if r.Chunk != nil {
			chunkID = r.Chunk.ID
			documentID = r.Chunk.DocumentID
			sourceID = r.Chunk.SourceID
			content = r.Chunk.Content
		}
		if r.Document != nil && documentID == "" {
			documentID = r.Document.ID
			sourceID = r.Document.SourceID
		}
		candidates[i] = &pipeline.Candidate{
			DocumentID: documentID,
			ChunkID:    chunkID,
			SourceID:   sourceID,
			Content:    content,
			Score:      r.Score,
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
