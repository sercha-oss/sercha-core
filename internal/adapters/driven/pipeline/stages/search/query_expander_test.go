package search

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// --- Mock LLM Service ---

type mockLLMService struct {
	response domain.CompletionResponse
	err      error
	called   bool
	lastReq  domain.CompletionRequest
}

func (m *mockLLMService) Complete(ctx context.Context, req domain.CompletionRequest) (domain.CompletionResponse, error) {
	m.called = true
	m.lastReq = req
	return m.response, m.err
}

func (m *mockLLMService) Model() string {
	return "test-model"
}

func (m *mockLLMService) Ping(ctx context.Context) error {
	return nil
}

func (m *mockLLMService) Close() error {
	return nil
}

// --- QueryExpanderFactory Tests ---

func TestQueryExpanderFactory_Descriptor(t *testing.T) {
	factory := NewQueryExpanderFactory()
	desc := factory.Descriptor()

	if desc.ID != QueryExpanderStageID {
		t.Errorf("ID = %q, want %q", desc.ID, QueryExpanderStageID)
	}
	if desc.Name != "Query Expander" {
		t.Errorf("Name = %q, want %q", desc.Name, "Query Expander")
	}
	if desc.Type != pipeline.StageTypeExpander {
		t.Errorf("Type = %q, want %q", desc.Type, pipeline.StageTypeExpander)
	}
	if desc.InputShape != pipeline.ShapeParsedQuery {
		t.Errorf("InputShape = %q, want %q", desc.InputShape, pipeline.ShapeParsedQuery)
	}
	if desc.OutputShape != pipeline.ShapeQuerySet {
		t.Errorf("OutputShape = %q, want %q", desc.OutputShape, pipeline.ShapeQuerySet)
	}
	if desc.Cardinality != pipeline.CardinalityOneToMany {
		t.Errorf("Cardinality = %q, want %q", desc.Cardinality, pipeline.CardinalityOneToMany)
	}

	// Verify capabilities
	if len(desc.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(desc.Capabilities))
	}
	if desc.Capabilities[0].Type != pipeline.CapabilityLLM {
		t.Errorf("capability type = %q, want %q", desc.Capabilities[0].Type, pipeline.CapabilityLLM)
	}
	if desc.Capabilities[0].Mode != pipeline.CapabilityOptional {
		t.Errorf("capability mode = %q, want %q", desc.Capabilities[0].Mode, pipeline.CapabilityOptional)
	}
}

func TestQueryExpanderFactory_Create_WithLLM(t *testing.T) {
	factory := NewQueryExpanderFactory()
	llm := &mockLLMService{}

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilityLLM, "test-llm", llm)

	config := pipeline.StageConfig{
		StageID: QueryExpanderStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	expander, ok := stage.(*QueryExpanderStage)
	if !ok {
		t.Fatalf("expected *QueryExpanderStage, got %T", stage)
	}

	if expander.llm == nil {
		t.Error("llm should be set when available in capability set")
	}
	if expander.maxVariants != 3 {
		t.Errorf("maxVariants = %d, want 3 (default)", expander.maxVariants)
	}
}

func TestQueryExpanderFactory_Create_WithoutLLM(t *testing.T) {
	factory := NewQueryExpanderFactory()
	capSet := pipeline.NewCapabilitySet()

	config := pipeline.StageConfig{
		StageID: QueryExpanderStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil (LLM is optional)", err)
	}

	expander, ok := stage.(*QueryExpanderStage)
	if !ok {
		t.Fatalf("expected *QueryExpanderStage, got %T", stage)
	}

	if expander.llm != nil {
		t.Error("llm should be nil when not available")
	}
}

func TestQueryExpanderFactory_Create_WithMaxVariantsParam(t *testing.T) {
	factory := NewQueryExpanderFactory()
	capSet := pipeline.NewCapabilitySet()

	config := pipeline.StageConfig{
		StageID: QueryExpanderStageID,
		Enabled: true,
		Parameters: map[string]any{
			"max_variants": float64(5),
		},
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	expander := stage.(*QueryExpanderStage)
	if expander.maxVariants != 5 {
		t.Errorf("maxVariants = %d, want 5", expander.maxVariants)
	}
}

// --- QueryExpanderStage Tests ---

func TestQueryExpanderStage_Process_WithLLM_ReturnsOriginalAndVariants(t *testing.T) {
	variants := []string{
		"python asyncio tutorial",
		"python concurrent programming",
	}
	variantsJSON := map[string]any{
		"queries": variants,
	}
	jsonBytes, _ := json.Marshal(variantsJSON)

	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: string(jsonBytes),
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "python async examples",
		Terms:    []string{"python", "async", "examples"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"src-1"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries, ok := result.([]*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("expected []*pipeline.ParsedQuery, got %T", result)
	}

	// Should have original + 2 variants = 3 total
	if len(queries) != 3 {
		t.Fatalf("len(queries) = %d, want 3 (original + 2 variants)", len(queries))
	}

	// First should be original
	if queries[0].Original != "python async examples" {
		t.Errorf("queries[0].Original = %q, want %q", queries[0].Original, "python async examples")
	}

	// Variants should match
	if queries[1].Original != variants[0] {
		t.Errorf("queries[1].Original = %q, want %q", queries[1].Original, variants[0])
	}
	if queries[2].Original != variants[1] {
		t.Errorf("queries[2].Original = %q, want %q", queries[2].Original, variants[1])
	}

	// Verify LLM was called
	if !llm.called {
		t.Error("LLM should have been called")
	}
}

func TestQueryExpanderStage_Process_LLMError_ReturnsOriginalOnly(t *testing.T) {
	llm := &mockLLMService{
		err: errors.New("LLM service unavailable"),
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (graceful degradation)", err)
	}

	queries, ok := result.([]*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("expected []*pipeline.ParsedQuery, got %T", result)
	}

	// Should only have original query
	if len(queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1 (original only on LLM error)", len(queries))
	}

	if queries[0].Original != "test query" {
		t.Errorf("queries[0].Original = %q, want %q", queries[0].Original, "test query")
	}
}

func TestQueryExpanderStage_Process_LLMReturnsEmpty_ReturnsOriginalOnly(t *testing.T) {
	emptyJSON := map[string]any{
		"queries": []string{},
	}
	jsonBytes, _ := json.Marshal(emptyJSON)

	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: string(jsonBytes),
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries := result.([]*pipeline.ParsedQuery)

	// Should only have original query when LLM returns empty
	if len(queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1 (original only when LLM returns empty)", len(queries))
	}
}

func TestQueryExpanderStage_Process_NoLLM_ReturnsOriginalOnly(t *testing.T) {
	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         nil, // No LLM configured
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"src-1", "src-2"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries, ok := result.([]*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("expected []*pipeline.ParsedQuery, got %T", result)
	}

	// Should only have original query (NoOp mode)
	if len(queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1 (NoOp when no LLM)", len(queries))
	}

	if queries[0].Original != "test query" {
		t.Errorf("queries[0].Original = %q, want %q", queries[0].Original, "test query")
	}

	// Verify filters are preserved
	if len(queries[0].SearchFilters.Sources) != 2 {
		t.Errorf("len(SearchFilters.Sources) = %d, want 2", len(queries[0].SearchFilters.Sources))
	}
}

func TestQueryExpanderStage_Process_VariantsInheritSearchFilters(t *testing.T) {
	variants := []string{
		"kubernetes deployment",
		"k8s rollout strategies",
	}
	variantsJSON := map[string]any{
		"queries": variants,
	}
	jsonBytes, _ := json.Marshal(variantsJSON)

	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: string(jsonBytes),
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "kubernetes deploy",
		Terms:    []string{"kubernetes", "deploy"},
		SearchFilters: pipeline.SearchFilters{
			Sources:      []string{"src-docs", "src-wiki"},
			ContentTypes: []string{"text/plain"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries := result.([]*pipeline.ParsedQuery)

	// All variants should inherit the search filters from original
	for i, q := range queries {
		if len(q.SearchFilters.Sources) != 2 {
			t.Errorf("queries[%d].SearchFilters.Sources length = %d, want 2", i, len(q.SearchFilters.Sources))
		}
		if q.SearchFilters.Sources[0] != "src-docs" || q.SearchFilters.Sources[1] != "src-wiki" {
			t.Errorf("queries[%d].SearchFilters.Sources = %v, want [src-docs, src-wiki]", i, q.SearchFilters.Sources)
		}
		if len(q.SearchFilters.ContentTypes) != 1 || q.SearchFilters.ContentTypes[0] != "text/plain" {
			t.Errorf("queries[%d].SearchFilters.ContentTypes = %v, want [text/plain]", i, q.SearchFilters.ContentTypes)
		}
	}
}

func TestQueryExpanderStage_Process_MaxVariantsRespected(t *testing.T) {
	// LLM returns 5 variants, but maxVariants is 3
	variants := []string{
		"variant 1",
		"variant 2",
		"variant 3",
		"variant 4",
		"variant 5",
	}
	variantsJSON := map[string]any{
		"queries": variants,
	}
	jsonBytes, _ := json.Marshal(variantsJSON)

	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: string(jsonBytes),
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3, // Limit to 3 variants
	}

	input := &pipeline.ParsedQuery{
		Original: "original query",
		Terms:    []string{"original", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries := result.([]*pipeline.ParsedQuery)

	// Should have original + 3 variants (max) = 4 total
	if len(queries) != 4 {
		t.Fatalf("len(queries) = %d, want 4 (original + 3 capped variants)", len(queries))
	}

	// Verify first 3 variants are included
	if queries[1].Original != variants[0] {
		t.Errorf("queries[1].Original = %q, want %q", queries[1].Original, variants[0])
	}
	if queries[2].Original != variants[1] {
		t.Errorf("queries[2].Original = %q, want %q", queries[2].Original, variants[1])
	}
	if queries[3].Original != variants[2] {
		t.Errorf("queries[3].Original = %q, want %q", queries[3].Original, variants[2])
	}
}

func TestQueryExpanderStage_Process_VariantsHaveTermsTokenized(t *testing.T) {
	variants := []string{
		"python asyncio tutorial",
	}
	variantsJSON := map[string]any{
		"queries": variants,
	}
	jsonBytes, _ := json.Marshal(variantsJSON)

	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: string(jsonBytes),
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "python async",
		Terms:    []string{"python", "async"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	queries := result.([]*pipeline.ParsedQuery)

	// Check variant has terms tokenized
	if len(queries) < 2 {
		t.Fatalf("expected at least 2 queries, got %d", len(queries))
	}

	variant := queries[1]
	expectedTerms := []string{"python", "asyncio", "tutorial"}
	if len(variant.Terms) != len(expectedTerms) {
		t.Fatalf("variant.Terms length = %d, want %d", len(variant.Terms), len(expectedTerms))
	}
	for i, term := range expectedTerms {
		if variant.Terms[i] != term {
			t.Errorf("variant.Terms[%d] = %q, want %q", i, variant.Terms[i], term)
		}
	}
}

func TestQueryExpanderStage_Process_InvalidInputType(t *testing.T) {
	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         nil,
		maxVariants: 3,
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
	if stageErr.Stage != QueryExpanderStageID {
		t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, QueryExpanderStageID)
	}
}

func TestQueryExpanderStage_Process_LLMInvalidJSON_ReturnsOriginalOnly(t *testing.T) {
	llm := &mockLLMService{
		response: domain.CompletionResponse{
			Content: "not valid json",
		},
	}

	stage := &QueryExpanderStage{
		descriptor:  NewQueryExpanderFactory().Descriptor(),
		llm:         llm,
		maxVariants: 3,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (graceful degradation)", err)
	}

	queries := result.([]*pipeline.ParsedQuery)

	// Should gracefully degrade to original only
	if len(queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1 (graceful degradation on invalid JSON)", len(queries))
	}
}

// --- Tokenizer Tests ---

func TestQueryExpanderStage_Tokenize(t *testing.T) {
	stage := &QueryExpanderStage{}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple words",
			input: "hello world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "with punctuation",
			input: "Hello, world!",
			want:  []string{"hello", "world"},
		},
		{
			name:  "multiple spaces",
			input: "hello    world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "mixed case",
			input: "Python Async Programming",
			want:  []string{"python", "async", "programming"},
		},
		{
			name:  "with special chars",
			input: "k8s-deployment yaml",
			want:  []string{"k8s-deployment", "yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stage.tokenize(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len(tokenize(%q)) = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
