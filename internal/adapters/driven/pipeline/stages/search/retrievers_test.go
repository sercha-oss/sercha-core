package search

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// --- Inline test doubles ---

type stubSearchEngine struct {
	lastQuery    string
	lastOpts     domain.SearchOptions
	docResults   []driven.DocumentResult
	docTotal     int
	searchCalled bool
}

func (s *stubSearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error { return nil }
func (s *stubSearchEngine) IndexDocument(ctx context.Context, doc *domain.DocumentContent) error {
	return nil
}
func (s *stubSearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	return nil, 0, nil
}
func (s *stubSearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	s.searchCalled = true
	s.lastQuery = query
	s.lastOpts = opts
	return s.docResults, s.docTotal, nil
}
func (s *stubSearchEngine) Delete(ctx context.Context, chunkIDs []string) error { return nil }
func (s *stubSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}
func (s *stubSearchEngine) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return nil
}
func (s *stubSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error { return nil }
func (s *stubSearchEngine) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return nil
}
func (s *stubSearchEngine) HealthCheck(ctx context.Context) error    { return nil }
func (s *stubSearchEngine) Count(ctx context.Context) (int64, error) { return 0, nil }
func (s *stubSearchEngine) GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error) {
	return nil, domain.ErrNotFound
}

type stubVectorIndex struct {
	lastEmbedding []float32
	lastK         int
	lastSourceIDs []string
	searchResults []driven.VectorSearchResult
	searchCalled  bool
}

func (s *stubVectorIndex) Index(ctx context.Context, id string, documentID string, embedding []float32) error {
	return nil
}
func (s *stubVectorIndex) IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error {
	return nil
}
func (s *stubVectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	return nil, nil, nil
}
func (s *stubVectorIndex) SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string) ([]driven.VectorSearchResult, error) {
	s.searchCalled = true
	s.lastEmbedding = embedding
	s.lastK = k
	s.lastSourceIDs = sourceIDs
	return s.searchResults, nil
}
func (s *stubVectorIndex) Delete(ctx context.Context, id string) error         { return nil }
func (s *stubVectorIndex) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (s *stubVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}
func (s *stubVectorIndex) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return nil
}
func (s *stubVectorIndex) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return nil
}
func (s *stubVectorIndex) HealthCheck(ctx context.Context) error { return nil }

type stubEmbedder struct {
	queryEmbedding []float32
}

func (s *stubEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}
func (s *stubEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return s.queryEmbedding, nil
}
func (s *stubEmbedder) Dimensions() int                       { return len(s.queryEmbedding) }
func (s *stubEmbedder) Model() string                         { return "test-model" }
func (s *stubEmbedder) HealthCheck(ctx context.Context) error { return nil }
func (s *stubEmbedder) Close() error                          { return nil }

// --- Query Parser Tests ---

func TestQueryParserStage_PropagatesSearchFilters(t *testing.T) {
	factory := NewQueryParserFactory()
	stage, err := factory.Create(pipeline.StageConfig{}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	input := &pipeline.SearchInput{
		Query: "test query",
		Filters: pipeline.SearchFilters{
			Sources:      []string{"src-1", "src-2"},
			ContentTypes: []string{"text/plain"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed := result.(*pipeline.ParsedQuery)
	if len(parsed.SearchFilters.Sources) != 2 {
		t.Fatalf("SearchFilters.Sources length = %d, want 2", len(parsed.SearchFilters.Sources))
	}
	if parsed.SearchFilters.Sources[0] != "src-1" || parsed.SearchFilters.Sources[1] != "src-2" {
		t.Errorf("SearchFilters.Sources = %v, want [src-1, src-2]", parsed.SearchFilters.Sources)
	}
	if len(parsed.SearchFilters.ContentTypes) != 1 || parsed.SearchFilters.ContentTypes[0] != "text/plain" {
		t.Errorf("SearchFilters.ContentTypes = %v, want [text/plain]", parsed.SearchFilters.ContentTypes)
	}
}

func TestQueryParserStage_EmptyFilters(t *testing.T) {
	factory := NewQueryParserFactory()
	stage, err := factory.Create(pipeline.StageConfig{}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	input := &pipeline.SearchInput{
		Query:   "hello world",
		Filters: pipeline.SearchFilters{},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed := result.(*pipeline.ParsedQuery)
	if len(parsed.SearchFilters.Sources) != 0 {
		t.Errorf("SearchFilters.Sources should be empty, got %v", parsed.SearchFilters.Sources)
	}
}

// --- BM25 Retriever Tests ---

func TestBM25Retriever_PassesSourceFilters(t *testing.T) {
	engine := &stubSearchEngine{
		docResults: []driven.DocumentResult{
			{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "body", Score: 1.0},
		},
		docTotal: 1,
	}

	stage := &BM25RetrieverStage{
		descriptor:   NewBM25RetrieverFactory().Descriptor(),
		searchEngine: engine,
		topK:         100,
	}

	parsed := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"src-1", "src-2"},
		},
	}

	_, err := stage.Process(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if !engine.searchCalled {
		t.Fatal("SearchDocuments was not called")
	}
	if len(engine.lastOpts.SourceIDs) != 2 {
		t.Fatalf("SourceIDs length = %d, want 2", len(engine.lastOpts.SourceIDs))
	}
	if engine.lastOpts.SourceIDs[0] != "src-1" || engine.lastOpts.SourceIDs[1] != "src-2" {
		t.Errorf("SourceIDs = %v, want [src-1, src-2]", engine.lastOpts.SourceIDs)
	}
}

func TestBM25Retriever_NoSourceFilters(t *testing.T) {
	engine := &stubSearchEngine{
		docResults: []driven.DocumentResult{},
		docTotal:   0,
	}

	stage := &BM25RetrieverStage{
		descriptor:   NewBM25RetrieverFactory().Descriptor(),
		searchEngine: engine,
		topK:         100,
	}

	parsed := &pipeline.ParsedQuery{
		Original:      "test query",
		Terms:         []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{},
	}

	_, err := stage.Process(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(engine.lastOpts.SourceIDs) != 0 {
		t.Errorf("SourceIDs should be empty, got %v", engine.lastOpts.SourceIDs)
	}
}

// --- Vector Retriever Tests ---

func TestVectorRetriever_PassesSourceFilters(t *testing.T) {
	vectorIdx := &stubVectorIndex{
		searchResults: []driven.VectorSearchResult{
			{ChunkID: "c1", DocumentID: "d1", Content: "chunk content", Distance: 0.1},
		},
	}
	embedder := &stubEmbedder{queryEmbedding: []float32{0.1, 0.2, 0.3}}

	stage := &VectorRetrieverStage{
		descriptor:  NewVectorRetrieverFactory().Descriptor(),
		vectorIndex: vectorIdx,
		embedder:    embedder,
		topK:        100,
	}

	parsed := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"src-A"},
		},
	}

	result, err := stage.Process(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if !vectorIdx.searchCalled {
		t.Fatal("SearchWithContent was not called")
	}
	if len(vectorIdx.lastSourceIDs) != 1 || vectorIdx.lastSourceIDs[0] != "src-A" {
		t.Errorf("lastSourceIDs = %v, want [src-A]", vectorIdx.lastSourceIDs)
	}

	candidates := result.([]*pipeline.Candidate)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Source != "vector" {
		t.Errorf("candidate source = %q, want %q", candidates[0].Source, "vector")
	}
}

func TestVectorRetriever_NoSourceFilters(t *testing.T) {
	vectorIdx := &stubVectorIndex{
		searchResults: []driven.VectorSearchResult{},
	}
	embedder := &stubEmbedder{queryEmbedding: []float32{0.1, 0.2, 0.3}}

	stage := &VectorRetrieverStage{
		descriptor:  NewVectorRetrieverFactory().Descriptor(),
		vectorIndex: vectorIdx,
		embedder:    embedder,
		topK:        100,
	}

	parsed := &pipeline.ParsedQuery{
		Original:      "test query",
		Terms:         []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{},
	}

	_, err := stage.Process(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(vectorIdx.lastSourceIDs) != 0 {
		t.Errorf("lastSourceIDs should be empty, got %v", vectorIdx.lastSourceIDs)
	}
}

// --- Hybrid Retriever Tests ---

func TestHybridRetriever_PassesSourceFiltersToBothBackends(t *testing.T) {
	engine := &stubSearchEngine{
		docResults: []driven.DocumentResult{
			{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "body", Score: 1.0},
		},
		docTotal: 1,
	}
	vectorIdx := &stubVectorIndex{
		searchResults: []driven.VectorSearchResult{
			{ChunkID: "c1", DocumentID: "d1", Content: "chunk", Distance: 0.15},
		},
	}
	embedder := &stubEmbedder{queryEmbedding: []float32{0.1, 0.2}}

	stage := &HybridRetrieverStage{
		descriptor:   NewHybridRetrieverFactory().Descriptor(),
		searchEngine: engine,
		vectorIndex:  vectorIdx,
		embedder:     embedder,
		topK:         50,
	}

	parsed := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"src-1", "src-2"},
		},
	}

	result, err := stage.Process(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify BM25 received source filters
	if !engine.searchCalled {
		t.Fatal("SearchDocuments was not called")
	}
	if len(engine.lastOpts.SourceIDs) != 2 {
		t.Errorf("BM25 SourceIDs length = %d, want 2", len(engine.lastOpts.SourceIDs))
	}

	// Verify vector received source filters
	if !vectorIdx.searchCalled {
		t.Fatal("SearchWithContent was not called")
	}
	if len(vectorIdx.lastSourceIDs) != 2 {
		t.Errorf("Vector sourceIDs length = %d, want 2", len(vectorIdx.lastSourceIDs))
	}

	// Verify we get candidates from both sources
	candidates := result.([]*pipeline.Candidate)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates (1 bm25 + 1 vector), got %d", len(candidates))
	}

	sources := map[string]bool{}
	for _, c := range candidates {
		sources[c.Source] = true
	}
	if !sources["bm25"] || !sources["vector"] {
		t.Errorf("expected both bm25 and vector sources, got %v", sources)
	}
}

// --- Conversion helpers ---

func TestConvertVectorResultsToCandidates_ScoreConversion(t *testing.T) {
	results := []driven.VectorSearchResult{
		{ChunkID: "c1", DocumentID: "d1", Content: "hello", Distance: 0.2},
		{ChunkID: "c2", DocumentID: "d2", Content: "world", Distance: 1.5},
	}

	candidates := convertVectorResultsToCandidates(results, "vector")

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Distance 0.2 → score 0.8
	if candidates[0].Score != 0.8 {
		t.Errorf("candidates[0].Score = %v, want 0.8", candidates[0].Score)
	}

	// Distance 1.5 → 1-1.5 = -0.5 → clamped to 0
	if candidates[1].Score != 0.0 {
		t.Errorf("candidates[1].Score = %v, want 0.0 (clamped)", candidates[1].Score)
	}

	if candidates[0].Source != "vector" {
		t.Errorf("source = %q, want %q", candidates[0].Source, "vector")
	}
}

func TestConvertDocResultsToCandidates(t *testing.T) {
	results := []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Test", Content: "body", Score: 5.5},
	}

	candidates := convertDocResultsToCandidates(results, "bm25")

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.DocumentID != "d1" {
		t.Errorf("DocumentID = %q, want d1", c.DocumentID)
	}
	if c.SourceID != "src-1" {
		t.Errorf("SourceID = %q, want src-1", c.SourceID)
	}
	if c.ChunkID != "" {
		t.Errorf("ChunkID should be empty for doc-level result, got %q", c.ChunkID)
	}
	if c.Score != 5.5 {
		t.Errorf("Score = %v, want 5.5", c.Score)
	}
	if c.Source != "bm25" {
		t.Errorf("Source = %q, want bm25", c.Source)
	}
}
