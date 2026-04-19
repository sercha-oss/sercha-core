package search

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// --- Mock SearchEngine with concurrency support ---

type mockSearchEngine struct {
	mu           sync.Mutex
	queryResults map[string][]driven.DocumentResult
	callCount    int
	queries      []string
	shouldError  bool
}

func newMockSearchEngine() *mockSearchEngine {
	return &mockSearchEngine{
		queryResults: make(map[string][]driven.DocumentResult),
		queries:      []string{},
	}
}

func (m *mockSearchEngine) setResults(query string, results []driven.DocumentResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryResults[query] = results
}

func (m *mockSearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	return nil
}

func (m *mockSearchEngine) IndexDocument(ctx context.Context, doc *domain.DocumentContent) error {
	return nil
}

func (m *mockSearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	return nil, 0, nil
}

func (m *mockSearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	m.queries = append(m.queries, query)

	if m.shouldError {
		return nil, 0, errors.New("search error")
	}

	results, ok := m.queryResults[query]
	if !ok {
		return []driven.DocumentResult{}, 0, nil
	}
	return results, len(results), nil
}

func (m *mockSearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	return nil
}

func (m *mockSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockSearchEngine) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return nil
}

func (m *mockSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	return nil
}

func (m *mockSearchEngine) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return nil
}

func (m *mockSearchEngine) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockSearchEngine) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockSearchEngine) GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error) {
	return nil, domain.ErrNotFound
}

// --- Mock VectorIndex with concurrency support ---

type mockVectorIndex struct {
	mu           sync.Mutex
	searchResult []driven.VectorSearchResult
	callCount    int
	shouldError  bool
}

func (m *mockVectorIndex) Index(ctx context.Context, id string, documentID string, embedding []float32) error {
	return nil
}

func (m *mockVectorIndex) IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error {
	return nil
}

func (m *mockVectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	return nil, nil, nil
}

func (m *mockVectorIndex) SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string, documentIDs []string) ([]driven.VectorSearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++

	if m.shouldError {
		return nil, errors.New("vector search error")
	}

	return m.searchResult, nil
}

func (m *mockVectorIndex) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorIndex) DeleteBatch(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockVectorIndex) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return nil
}

func (m *mockVectorIndex) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return nil
}

func (m *mockVectorIndex) HealthCheck(ctx context.Context) error {
	return nil
}

// --- Mock EmbeddingService ---

type mockEmbedder struct {
	embedding   []float32
	shouldError bool
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	if m.shouldError {
		return nil, errors.New("embedding error")
	}
	return m.embedding, nil
}

func (m *mockEmbedder) Dimensions() int {
	return len(m.embedding)
}

func (m *mockEmbedder) Model() string {
	return "test-embedder"
}

func (m *mockEmbedder) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockEmbedder) Close() error {
	return nil
}

// --- MultiRetrieverFactory Tests ---

func TestMultiRetrieverFactory_Descriptor(t *testing.T) {
	factory := NewMultiRetrieverFactory()
	desc := factory.Descriptor()

	if desc.ID != MultiRetrieverStageID {
		t.Errorf("ID = %q, want %q", desc.ID, MultiRetrieverStageID)
	}
	if desc.Name != "Multi-Query Retriever" {
		t.Errorf("Name = %q, want %q", desc.Name, "Multi-Query Retriever")
	}
	if desc.Type != pipeline.StageTypeRetriever {
		t.Errorf("Type = %q, want %q", desc.Type, pipeline.StageTypeRetriever)
	}
	if desc.InputShape != pipeline.ShapeQuerySet {
		t.Errorf("InputShape = %q, want %q", desc.InputShape, pipeline.ShapeQuerySet)
	}
	if desc.OutputShape != pipeline.ShapeCandidate {
		t.Errorf("OutputShape = %q, want %q", desc.OutputShape, pipeline.ShapeCandidate)
	}
	if desc.Cardinality != pipeline.CardinalityManyToOne {
		t.Errorf("Cardinality = %q, want %q", desc.Cardinality, pipeline.CardinalityManyToOne)
	}

	// Verify capabilities
	if len(desc.Capabilities) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(desc.Capabilities))
	}

	capMap := make(map[pipeline.CapabilityType]pipeline.CapabilityMode)
	for _, cap := range desc.Capabilities {
		capMap[cap.Type] = cap.Mode
	}

	if capMap[pipeline.CapabilitySearchEngine] != pipeline.CapabilityRequired {
		t.Errorf("search_engine capability mode = %q, want %q", capMap[pipeline.CapabilitySearchEngine], pipeline.CapabilityRequired)
	}
	if capMap[pipeline.CapabilityVectorStore] != pipeline.CapabilityOptional {
		t.Errorf("vector_store capability mode = %q, want %q", capMap[pipeline.CapabilityVectorStore], pipeline.CapabilityOptional)
	}
	if capMap[pipeline.CapabilityEmbedder] != pipeline.CapabilityOptional {
		t.Errorf("embedder capability mode = %q, want %q", capMap[pipeline.CapabilityEmbedder], pipeline.CapabilityOptional)
	}
}

func TestMultiRetrieverFactory_Create_RequiresSearchEngine(t *testing.T) {
	factory := NewMultiRetrieverFactory()
	capSet := pipeline.NewCapabilitySet()

	config := pipeline.StageConfig{
		StageID: MultiRetrieverStageID,
		Enabled: true,
	}

	_, err := factory.Create(config, capSet)
	if err == nil {
		t.Fatal("Create() error = nil, want error when search_engine not available")
	}

	stageErr, ok := err.(*StageError)
	if !ok {
		t.Fatalf("expected *StageError, got %T", err)
	}
	if stageErr.Stage != MultiRetrieverStageID {
		t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, MultiRetrieverStageID)
	}
}

func TestMultiRetrieverFactory_Create_BM25Only(t *testing.T) {
	factory := NewMultiRetrieverFactory()
	searchEngine := newMockSearchEngine()

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilitySearchEngine, "test-search", searchEngine)

	config := pipeline.StageConfig{
		StageID: MultiRetrieverStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	retriever, ok := stage.(*MultiRetrieverStage)
	if !ok {
		t.Fatalf("expected *MultiRetrieverStage, got %T", stage)
	}

	if retriever.searchEngine == nil {
		t.Error("searchEngine should be set")
	}
	if retriever.vectorIndex != nil {
		t.Error("vectorIndex should be nil in BM25-only mode")
	}
	if retriever.embedder != nil {
		t.Error("embedder should be nil in BM25-only mode")
	}
	if retriever.topK != DefaultTopK {
		t.Errorf("topK = %d, want %d (default)", retriever.topK, DefaultTopK)
	}
	if retriever.rrfK != DefaultRRFK {
		t.Errorf("rrfK = %d, want %d (default)", retriever.rrfK, DefaultRRFK)
	}
}

func TestMultiRetrieverFactory_Create_HybridMode(t *testing.T) {
	factory := NewMultiRetrieverFactory()
	searchEngine := newMockSearchEngine()
	vectorIdx := &mockVectorIndex{}
	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2}}

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilitySearchEngine, "test-search", searchEngine)
	capSet.Add(pipeline.CapabilityVectorStore, "test-vector", vectorIdx)
	capSet.Add(pipeline.CapabilityEmbedder, "test-embedder", embedder)

	config := pipeline.StageConfig{
		StageID: MultiRetrieverStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	retriever := stage.(*MultiRetrieverStage)

	if retriever.searchEngine == nil {
		t.Error("searchEngine should be set")
	}
	if retriever.vectorIndex == nil {
		t.Error("vectorIndex should be set in hybrid mode")
	}
	if retriever.embedder == nil {
		t.Error("embedder should be set in hybrid mode")
	}
}

func TestMultiRetrieverFactory_Create_WithParameters(t *testing.T) {
	factory := NewMultiRetrieverFactory()
	searchEngine := newMockSearchEngine()

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilitySearchEngine, "test-search", searchEngine)

	config := pipeline.StageConfig{
		StageID: MultiRetrieverStageID,
		Enabled: true,
		Parameters: map[string]any{
			"top_k": float64(50),
			"rrf_k": float64(100),
		},
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	retriever := stage.(*MultiRetrieverStage)

	if retriever.topK != 50 {
		t.Errorf("topK = %d, want 50", retriever.topK)
	}
	if retriever.rrfK != 100 {
		t.Errorf("rrfK = %d, want 100", retriever.rrfK)
	}
}

// --- MultiRetrieverStage Tests ---

func TestMultiRetrieverStage_Process_SingleQuery(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test query", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
		{DocumentID: "d2", SourceID: "src-1", Title: "Doc 2", Content: "content 2", Score: 3.0},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test query", Terms: []string{"test", "query"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("expected []*pipeline.Candidate, got %T", result)
	}

	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}

	// Verify candidates have RRF metadata
	for i, c := range candidates {
		if c.Metadata == nil {
			t.Errorf("candidates[%d].Metadata is nil", i)
			continue
		}
		if _, ok := c.Metadata["query_variants"]; !ok {
			t.Errorf("candidates[%d].Metadata missing query_variants", i)
		}
		if _, ok := c.Metadata["rrf_score"]; !ok {
			t.Errorf("candidates[%d].Metadata missing rrf_score", i)
		}
	}
}

func TestMultiRetrieverStage_Process_MultipleQueriesParallel(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("query 1", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})
	searchEngine.setResults("query 2", []driven.DocumentResult{
		{DocumentID: "d2", SourceID: "src-1", Title: "Doc 2", Content: "content 2", Score: 4.0},
	})
	searchEngine.setResults("query 3", []driven.DocumentResult{
		{DocumentID: "d3", SourceID: "src-1", Title: "Doc 3", Content: "content 3", Score: 3.0},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "query 1", Terms: []string{"query", "1"}},
		{Original: "query 2", Terms: []string{"query", "2"}},
		{Original: "query 3", Terms: []string{"query", "3"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should get 3 unique documents
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3", len(candidates))
	}

	// Verify all 3 queries were called (parallel execution)
	if searchEngine.callCount != 3 {
		t.Errorf("search call count = %d, want 3 (parallel execution)", searchEngine.callCount)
	}
}

func TestMultiRetrieverStage_Process_RRF_OriginalQueryWeightedHigher(t *testing.T) {
	searchEngine := newMockSearchEngine()

	// Original query finds d1 first
	searchEngine.setResults("original", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
		{DocumentID: "d2", SourceID: "src-1", Title: "Doc 2", Content: "content 2", Score: 4.0},
	})

	// Variant finds d2 first, d1 second
	searchEngine.setResults("variant", []driven.DocumentResult{
		{DocumentID: "d2", SourceID: "src-1", Title: "Doc 2", Content: "content 2", Score: 6.0},
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 3.0},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         60,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "original", Terms: []string{"original"}}, // Weight 1.0
		{Original: "variant", Terms: []string{"variant"}},   // Weight 0.8
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}

	// d1 should rank higher because original query (weight 1.0) ranked it first
	// d1 RRF: 1.0 * 1/(60+0+1) + 0.8 * 1/(60+1+1) ≈ 0.0164 + 0.0129 ≈ 0.0293
	// d2 RRF: 1.0 * 1/(60+1+1) + 0.8 * 1/(60+0+1) ≈ 0.0161 + 0.0131 ≈ 0.0292
	// d1 should rank slightly higher due to higher weight on position 0

	if candidates[0].DocumentID != "d1" {
		t.Errorf("candidates[0].DocumentID = %q, want d1 (original query weighted higher)", candidates[0].DocumentID)
	}
	if candidates[1].DocumentID != "d2" {
		t.Errorf("candidates[1].DocumentID = %q, want d2", candidates[1].DocumentID)
	}

	// Verify d1 has higher score
	if candidates[0].Score <= candidates[1].Score {
		t.Errorf("candidates[0].Score (%f) should be > candidates[1].Score (%f)", candidates[0].Score, candidates[1].Score)
	}
}

func TestMultiRetrieverStage_Process_RRF_Deduplication(t *testing.T) {
	searchEngine := newMockSearchEngine()

	// Both queries return the same document
	searchEngine.setResults("query 1", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})
	searchEngine.setResults("query 2", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 4.5},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "query 1", Terms: []string{"query", "1"}},
		{Original: "query 2", Terms: []string{"query", "2"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should deduplicate to single document
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1 (deduplicated)", len(candidates))
	}

	if candidates[0].DocumentID != "d1" {
		t.Errorf("candidates[0].DocumentID = %q, want d1", candidates[0].DocumentID)
	}

	// Verify metadata shows both variants found it
	variants, ok := candidates[0].Metadata["query_variants"].([]int)
	if !ok {
		t.Fatalf("query_variants metadata not found or wrong type")
	}
	if len(variants) != 2 {
		t.Errorf("len(query_variants) = %d, want 2", len(variants))
	}
	if variants[0] != 0 || variants[1] != 1 {
		t.Errorf("query_variants = %v, want [0, 1]", variants)
	}
}

func TestMultiRetrieverStage_Process_MetadataContainsQueryVariantsAndRRFScore(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}

	c := candidates[0]

	// Verify query_variants metadata
	variants, ok := c.Metadata["query_variants"]
	if !ok {
		t.Error("Metadata missing query_variants")
	}
	variantsSlice, ok := variants.([]int)
	if !ok {
		t.Errorf("query_variants type = %T, want []int", variants)
	} else if len(variantsSlice) != 1 || variantsSlice[0] != 0 {
		t.Errorf("query_variants = %v, want [0]", variantsSlice)
	}

	// Verify rrf_score metadata
	rrfScore, ok := c.Metadata["rrf_score"]
	if !ok {
		t.Error("Metadata missing rrf_score")
	}
	if rrfScore == nil {
		t.Error("rrf_score is nil")
	}

	// Verify Score is set to RRF score
	if c.Score != rrfScore {
		t.Errorf("Score (%f) != rrf_score (%f)", c.Score, rrfScore)
	}
}

func TestMultiRetrieverStage_Process_BM25OnlyMode(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		vectorIndex:  nil, // BM25-only
		embedder:     nil,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}

	// Should have BM25 source only
	if candidates[0].Source != "bm25" {
		t.Errorf("candidates[0].Source = %q, want bm25", candidates[0].Source)
	}
}

func TestMultiRetrieverStage_Process_HybridMode(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})

	vectorIdx := &mockVectorIndex{
		searchResult: []driven.VectorSearchResult{
			{ChunkID: "c2", DocumentID: "d2", Content: "content 2", Distance: 0.2},
		},
	}

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3},
	}

	stage := &MultiRetrieverStage{
		descriptor:              NewMultiRetrieverFactory().Descriptor(),
		searchEngine:            searchEngine,
		vectorIndex:             vectorIdx,
		embedder:                embedder,
		topK:                    100,
		rrfK:                    DefaultRRFK,
		vectorDistanceThreshold: DefaultVectorDistanceThreshold,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should get results from both BM25 and vector
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2 (BM25 + vector)", len(candidates))
	}

	// Verify we have both sources
	sources := make(map[string]bool)
	for _, c := range candidates {
		sources[c.Source] = true
	}
	if !sources["bm25"] {
		t.Error("missing bm25 source in hybrid mode")
	}
	if !sources["vector"] {
		t.Error("missing vector source in hybrid mode")
	}
}

func TestMultiRetrieverStage_Process_EmptyResultsFromVariant(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("query 1", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})
	// query 2 returns empty results
	searchEngine.setResults("query 2", []driven.DocumentResult{})

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "query 1", Terms: []string{"query", "1"}},
		{Original: "query 2", Terms: []string{"query", "2"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should still return d1 from first variant
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].DocumentID != "d1" {
		t.Errorf("candidates[0].DocumentID = %q, want d1", candidates[0].DocumentID)
	}
}

func TestMultiRetrieverStage_Process_PartialFailures(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("query 1", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})
	// query 2 will error due to mock setup
	searchEngine.shouldError = false // Set to false initially
	searchEngine.setResults("query 2", []driven.DocumentResult{
		{DocumentID: "d2", SourceID: "src-1", Title: "Doc 2", Content: "content 2", Score: 4.0},
	})

	// Use a custom mock that can fail selectively
	selectiveEngine := &selectiveErrorSearchEngine{
		base:        searchEngine,
		failOnQuery: "query 2",
	}

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: selectiveEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "query 1", Terms: []string{"query", "1"}},
		{Original: "query 2", Terms: []string{"query", "2"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (graceful handling of partial failures)", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should still return d1 from successful variant
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1 (from successful variant)", len(candidates))
	}
	if candidates[0].DocumentID != "d1" {
		t.Errorf("candidates[0].DocumentID = %q, want d1", candidates[0].DocumentID)
	}
}

func TestMultiRetrieverStage_Process_EmptyInput(t *testing.T) {
	searchEngine := newMockSearchEngine()

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("expected []*pipeline.Candidate, got %T", result)
	}

	if len(candidates) != 0 {
		t.Errorf("len(candidates) = %d, want 0 for empty input", len(candidates))
	}
}

func TestMultiRetrieverStage_Process_InvalidInputType(t *testing.T) {
	searchEngine := newMockSearchEngine()

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	// Pass wrong input type
	input := "invalid input"

	_, err := stage.Process(context.Background(), input)
	if err == nil {
		t.Fatal("Process() error = nil, want error for invalid input type")
	}

	stageErr, ok := err.(*StageError)
	if !ok {
		t.Fatalf("expected *StageError, got %T", err)
	}
	if stageErr.Stage != MultiRetrieverStageID {
		t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, MultiRetrieverStageID)
	}
}

func TestMultiRetrieverStage_Process_TopKLimit(t *testing.T) {
	searchEngine := newMockSearchEngine()

	// Return many results
	results := make([]driven.DocumentResult, 50)
	for i := 0; i < 50; i++ {
		results[i] = driven.DocumentResult{
			DocumentID: "doc-" + string(rune('0'+i)),
			SourceID:   "src-1",
			Title:      "Doc",
			Content:    "content",
			Score:      float64(50 - i),
		}
	}
	searchEngine.setResults("test", results)

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		topK:         10, // Limit to 10
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should respect topK limit
	if len(candidates) != 10 {
		t.Fatalf("len(candidates) = %d, want 10 (topK limit)", len(candidates))
	}
}

func TestMultiRetrieverStage_Process_VectorSearchError_ContinuesWithBM25(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})

	vectorIdx := &mockVectorIndex{
		shouldError: true, // Vector search will fail
	}

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3},
	}

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		vectorIndex:  vectorIdx,
		embedder:     embedder,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (graceful degradation)", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should still get BM25 results
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1 (BM25 only after vector error)", len(candidates))
	}
	if candidates[0].Source != "bm25" {
		t.Errorf("candidates[0].Source = %q, want bm25", candidates[0].Source)
	}
}

func TestMultiRetrieverStage_Process_EmbedderError_ContinuesWithBM25(t *testing.T) {
	searchEngine := newMockSearchEngine()
	searchEngine.setResults("test", []driven.DocumentResult{
		{DocumentID: "d1", SourceID: "src-1", Title: "Doc 1", Content: "content 1", Score: 5.0},
	})

	vectorIdx := &mockVectorIndex{
		searchResult: []driven.VectorSearchResult{},
	}

	embedder := &mockEmbedder{
		shouldError: true, // Embedding will fail
	}

	stage := &MultiRetrieverStage{
		descriptor:   NewMultiRetrieverFactory().Descriptor(),
		searchEngine: searchEngine,
		vectorIndex:  vectorIdx,
		embedder:     embedder,
		topK:         100,
		rrfK:         DefaultRRFK,
	}

	input := []*pipeline.ParsedQuery{
		{Original: "test", Terms: []string{"test"}},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (graceful degradation)", err)
	}

	candidates := result.([]*pipeline.Candidate)

	// Should still get BM25 results
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1 (BM25 only after embedder error)", len(candidates))
	}
	if candidates[0].Source != "bm25" {
		t.Errorf("candidates[0].Source = %q, want bm25", candidates[0].Source)
	}
}

// --- Helper mock for selective errors ---

type selectiveErrorSearchEngine struct {
	base        *mockSearchEngine
	failOnQuery string
}

func (s *selectiveErrorSearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	return s.base.Index(ctx, chunks)
}

func (s *selectiveErrorSearchEngine) IndexDocument(ctx context.Context, doc *domain.DocumentContent) error {
	return s.base.IndexDocument(ctx, doc)
}

func (s *selectiveErrorSearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	return s.base.Search(ctx, query, queryEmbedding, opts)
}

func (s *selectiveErrorSearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	if query == s.failOnQuery {
		return nil, 0, errors.New("selective search error")
	}
	return s.base.SearchDocuments(ctx, query, opts)
}

func (s *selectiveErrorSearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	return s.base.Delete(ctx, chunkIDs)
}

func (s *selectiveErrorSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	return s.base.DeleteByDocument(ctx, documentID)
}

func (s *selectiveErrorSearchEngine) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	return s.base.DeleteByDocuments(ctx, documentIDs)
}

func (s *selectiveErrorSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	return s.base.DeleteBySource(ctx, sourceID)
}

func (s *selectiveErrorSearchEngine) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	return s.base.DeleteBySourceAndContainer(ctx, sourceID, containerID)
}

func (s *selectiveErrorSearchEngine) HealthCheck(ctx context.Context) error {
	return s.base.HealthCheck(ctx)
}

func (s *selectiveErrorSearchEngine) Count(ctx context.Context) (int64, error) {
	return s.base.Count(ctx)
}

func (s *selectiveErrorSearchEngine) GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error) {
	return s.base.GetDocument(ctx, documentID)
}
