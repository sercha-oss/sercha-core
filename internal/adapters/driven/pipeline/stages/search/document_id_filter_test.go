package search

import (
	"context"
	"errors"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// --- Mock DocumentIDProvider ---

type mockDocumentIDProvider struct {
	documentIDs []string
	err         error
	called      bool
	lastQuery   string
	lastFilters pipeline.SearchFilters
}

func (m *mockDocumentIDProvider) GetAllowedDocumentIDs(ctx context.Context, query string, filters pipeline.SearchFilters) ([]string, error) {
	m.called = true
	m.lastQuery = query
	m.lastFilters = filters
	return m.documentIDs, m.err
}

// --- DocumentIDFilterFactory Tests ---

func TestDocumentIDFilterFactory_StageID(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	if factory.StageID() != DocumentIDFilterStageID {
		t.Errorf("StageID() = %q, want %q", factory.StageID(), DocumentIDFilterStageID)
	}
}

func TestDocumentIDFilterFactory_Descriptor(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	desc := factory.Descriptor()

	if desc.ID != DocumentIDFilterStageID {
		t.Errorf("ID = %q, want %q", desc.ID, DocumentIDFilterStageID)
	}
	if desc.Name != "Document ID Filter" {
		t.Errorf("Name = %q, want %q", desc.Name, "Document ID Filter")
	}
	if desc.Type != pipeline.StageTypeParser {
		t.Errorf("Type = %q, want %q", desc.Type, pipeline.StageTypeParser)
	}
	if desc.InputShape != pipeline.ShapeParsedQuery {
		t.Errorf("InputShape = %q, want %q", desc.InputShape, pipeline.ShapeParsedQuery)
	}
	if desc.OutputShape != pipeline.ShapeParsedQuery {
		t.Errorf("OutputShape = %q, want %q", desc.OutputShape, pipeline.ShapeParsedQuery)
	}
	if desc.Cardinality != pipeline.CardinalityOneToOne {
		t.Errorf("Cardinality = %q, want %q", desc.Cardinality, pipeline.CardinalityOneToOne)
	}
	if desc.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", desc.Version)
	}

	// Verify capabilities
	if len(desc.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(desc.Capabilities))
	}
	if desc.Capabilities[0].Type != pipeline.CapabilityDocumentIDProvider {
		t.Errorf("capability type = %q, want %q", desc.Capabilities[0].Type, pipeline.CapabilityDocumentIDProvider)
	}
	if desc.Capabilities[0].Mode != pipeline.CapabilityOptional {
		t.Errorf("capability mode = %q, want %q", desc.Capabilities[0].Mode, pipeline.CapabilityOptional)
	}
}

func TestDocumentIDFilterFactory_Validate(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	config := pipeline.StageConfig{
		StageID: DocumentIDFilterStageID,
		Enabled: true,
	}

	if err := factory.Validate(config); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestDocumentIDFilterFactory_Create_WithProvider(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	provider := &mockDocumentIDProvider{
		documentIDs: []string{"doc-1", "doc-2"},
	}

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilityDocumentIDProvider, "test-provider", provider)

	config := pipeline.StageConfig{
		StageID: DocumentIDFilterStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	filterStage, ok := stage.(*DocumentIDFilterStage)
	if !ok {
		t.Fatalf("stage should be *DocumentIDFilterStage, got %T", stage)
	}
	if filterStage.provider == nil {
		t.Error("provider should be set when available")
	}
	if filterStage.Descriptor().ID != factory.Descriptor().ID {
		t.Error("stage descriptor should match factory descriptor")
	}
}

func TestDocumentIDFilterFactory_Create_WithoutProvider(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	capSet := pipeline.NewCapabilitySet()

	config := pipeline.StageConfig{
		StageID: DocumentIDFilterStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil (capability is optional)", err)
	}
	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	filterStage, ok := stage.(*DocumentIDFilterStage)
	if !ok {
		t.Fatalf("stage should be *DocumentIDFilterStage, got %T", stage)
	}
	if filterStage.provider != nil {
		t.Error("provider should be nil when not available")
	}
}

func TestDocumentIDFilterFactory_Create_WithWrongProviderType(t *testing.T) {
	factory := NewDocumentIDFilterFactory()

	// Add a capability with wrong type
	wrongProvider := "not a DocumentIDProvider"
	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilityDocumentIDProvider, "wrong-provider", wrongProvider)

	config := pipeline.StageConfig{
		StageID: DocumentIDFilterStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil (type assertion fails silently)", err)
	}
	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	filterStage, ok := stage.(*DocumentIDFilterStage)
	if !ok {
		t.Fatalf("stage should be *DocumentIDFilterStage, got %T", stage)
	}
	if filterStage.provider != nil {
		t.Error("provider should be nil when type assertion fails")
	}
}

// --- DocumentIDFilterStage Tests ---

func TestDocumentIDFilterStage_Descriptor(t *testing.T) {
	factory := NewDocumentIDFilterFactory()
	stage := &DocumentIDFilterStage{
		descriptor: factory.Descriptor(),
	}

	desc := stage.Descriptor()
	if desc.ID != DocumentIDFilterStageID {
		t.Errorf("ID = %q, want %q", desc.ID, DocumentIDFilterStageID)
	}
	if desc.Name != "Document ID Filter" {
		t.Errorf("Name = %q, want Document ID Filter", desc.Name)
	}
}

func TestDocumentIDFilterStage_Process_WithProvider_PopulatesDocumentIDs(t *testing.T) {
	provider := &mockDocumentIDProvider{
		documentIDs: []string{"doc-1", "doc-2", "doc-3"},
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources: []string{"source-1"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result == nil {
		t.Fatal("Process() returned nil result")
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	// Verify document IDs were populated
	if len(parsed.SearchFilters.DocumentIDs) != 3 {
		t.Errorf("len(DocumentIDs) = %d, want 3", len(parsed.SearchFilters.DocumentIDs))
	}
	for i, want := range []string{"doc-1", "doc-2", "doc-3"} {
		if parsed.SearchFilters.DocumentIDs[i] != want {
			t.Errorf("DocumentIDs[%d] = %q, want %q", i, parsed.SearchFilters.DocumentIDs[i], want)
		}
	}

	// Verify provider was called with correct arguments
	if !provider.called {
		t.Error("provider should have been called")
	}
	if provider.lastQuery != "test query" {
		t.Errorf("provider lastQuery = %q, want %q", provider.lastQuery, "test query")
	}
	if len(provider.lastFilters.Sources) != 1 || provider.lastFilters.Sources[0] != "source-1" {
		t.Errorf("provider lastFilters.Sources = %v, want [source-1]", provider.lastFilters.Sources)
	}
}

func TestDocumentIDFilterStage_Process_WithProvider_EmptySlice(t *testing.T) {
	provider := &mockDocumentIDProvider{
		documentIDs: []string{}, // Empty slice means no filtering
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	// Empty slice should be set (downstream can check for nil vs empty)
	if parsed.SearchFilters.DocumentIDs == nil {
		t.Error("DocumentIDs should not be nil, should be empty slice")
	}
	if len(parsed.SearchFilters.DocumentIDs) != 0 {
		t.Errorf("len(DocumentIDs) = %d, want 0", len(parsed.SearchFilters.DocumentIDs))
	}
	if !provider.called {
		t.Error("provider should have been called")
	}
}

func TestDocumentIDFilterStage_Process_WithProvider_NilSlice(t *testing.T) {
	provider := &mockDocumentIDProvider{
		documentIDs: nil, // Nil means no filtering
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	// Nil slice should remain nil
	if parsed.SearchFilters.DocumentIDs != nil {
		t.Error("DocumentIDs should be nil")
	}
	if !provider.called {
		t.Error("provider should have been called")
	}
}

func TestDocumentIDFilterStage_Process_NoProvider_PassThrough(t *testing.T) {
	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   nil, // No provider available
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		SearchFilters: pipeline.SearchFilters{
			Sources:      []string{"source-1"},
			ContentTypes: []string{"text/plain"},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result == nil {
		t.Fatal("Process() returned nil result")
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	// Should pass through unchanged
	if parsed.Original != "test query" {
		t.Errorf("Original = %q, want %q", parsed.Original, "test query")
	}
	if len(parsed.Terms) != 2 {
		t.Errorf("len(Terms) = %d, want 2", len(parsed.Terms))
	}
	if len(parsed.SearchFilters.Sources) != 1 {
		t.Errorf("len(Sources) = %d, want 1", len(parsed.SearchFilters.Sources))
	}
	if parsed.SearchFilters.DocumentIDs != nil {
		t.Error("DocumentIDs should remain nil")
	}
}

func TestDocumentIDFilterStage_Process_ProviderError_ReturnsStageError(t *testing.T) {
	providerErr := errors.New("database connection failed")
	provider := &mockDocumentIDProvider{
		err: providerErr,
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err == nil {
		t.Fatal("Process() error = nil, want error")
	}
	if result != nil {
		t.Errorf("result should be nil on error, got %T", result)
	}

	// Verify it's a StageError
	stageErr, ok := err.(*StageError)
	if !ok {
		t.Fatalf("error should be *StageError, got %T", err)
	}
	if stageErr.Stage != DocumentIDFilterStageID {
		t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, DocumentIDFilterStageID)
	}
	if stageErr.Message != "failed to get allowed document IDs" {
		t.Errorf("StageError.Message = %q, want %q", stageErr.Message, "failed to get allowed document IDs")
	}
	if stageErr.Err != providerErr {
		t.Errorf("StageError.Err = %v, want %v", stageErr.Err, providerErr)
	}
}

func TestDocumentIDFilterStage_Process_InvalidInputType(t *testing.T) {
	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   nil,
	}

	// Pass wrong input type
	invalidInputs := []any{
		"string input",
		123,
		[]string{"slice"},
		map[string]string{"key": "value"},
		nil,
	}

	for _, input := range invalidInputs {
		t.Run("invalid_input", func(t *testing.T) {
			result, err := stage.Process(context.Background(), input)
			if err == nil {
				t.Fatal("Process() error = nil, want error for invalid input")
			}
			if result != nil {
				t.Error("result should be nil on error")
			}

			stageErr, ok := err.(*StageError)
			if !ok {
				t.Fatalf("error should be *StageError, got %T", err)
			}
			if stageErr.Stage != DocumentIDFilterStageID {
				t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, DocumentIDFilterStageID)
			}
			if stageErr.Message != "expected *pipeline.ParsedQuery" {
				t.Errorf("StageError.Message = %q, want %q", stageErr.Message, "expected *pipeline.ParsedQuery")
			}
		})
	}
}

func TestDocumentIDFilterStage_Process_PreservesExistingFilters(t *testing.T) {
	provider := &mockDocumentIDProvider{
		documentIDs: []string{"doc-1", "doc-2"},
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
		Phrases:  []string{"test phrase"},
		SearchFilters: pipeline.SearchFilters{
			Sources:      []string{"source-1", "source-2"},
			ContentTypes: []string{"text/plain", "text/markdown"},
			Custom: map[string]any{
				"custom_field": "custom_value",
			},
		},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	// Verify all original data is preserved
	if parsed.Original != "test query" {
		t.Errorf("Original = %q, want %q", parsed.Original, "test query")
	}
	if len(parsed.Terms) != 2 {
		t.Errorf("len(Terms) = %d, want 2", len(parsed.Terms))
	}
	if len(parsed.Phrases) != 1 {
		t.Errorf("len(Phrases) = %d, want 1", len(parsed.Phrases))
	}
	if len(parsed.SearchFilters.Sources) != 2 {
		t.Errorf("len(Sources) = %d, want 2", len(parsed.SearchFilters.Sources))
	}
	if len(parsed.SearchFilters.ContentTypes) != 2 {
		t.Errorf("len(ContentTypes) = %d, want 2", len(parsed.SearchFilters.ContentTypes))
	}
	if parsed.SearchFilters.Custom["custom_field"] != "custom_value" {
		t.Errorf("Custom[custom_field] = %v, want custom_value", parsed.SearchFilters.Custom["custom_field"])
	}

	// And document IDs were added
	if len(parsed.SearchFilters.DocumentIDs) != 2 {
		t.Errorf("len(DocumentIDs) = %d, want 2", len(parsed.SearchFilters.DocumentIDs))
	}
}

func TestDocumentIDFilterStage_Process_LargeDocumentIDSet(t *testing.T) {
	// Test with 10k+ document IDs (common in ACL/tenant filtering)
	largeSet := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		largeSet[i] = "doc-" + string(rune(i))
	}

	provider := &mockDocumentIDProvider{
		documentIDs: largeSet,
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "test query",
		Terms:    []string{"test", "query"},
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	parsed, ok := result.(*pipeline.ParsedQuery)
	if !ok {
		t.Fatalf("result should be *pipeline.ParsedQuery, got %T", result)
	}

	if len(parsed.SearchFilters.DocumentIDs) != 10000 {
		t.Errorf("len(DocumentIDs) = %d, want 10000", len(parsed.SearchFilters.DocumentIDs))
	}
}

func TestDocumentIDFilterStage_Process_ProviderReceivesFilters(t *testing.T) {
	provider := &mockDocumentIDProvider{
		documentIDs: []string{"doc-1"},
	}

	stage := &DocumentIDFilterStage{
		descriptor: NewDocumentIDFilterFactory().Descriptor(),
		provider:   provider,
	}

	input := &pipeline.ParsedQuery{
		Original: "search for documents",
		Terms:    []string{"search", "for", "documents"},
		SearchFilters: pipeline.SearchFilters{
			Sources:      []string{"github", "confluence"},
			ContentTypes: []string{"text/markdown"},
			Custom: map[string]any{
				"team": "engineering",
			},
		},
	}

	_, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify provider received the correct query and filters
	if provider.lastQuery != "search for documents" {
		t.Errorf("provider lastQuery = %q, want %q", provider.lastQuery, "search for documents")
	}
	if len(provider.lastFilters.Sources) != 2 {
		t.Errorf("len(provider.lastFilters.Sources) = %d, want 2", len(provider.lastFilters.Sources))
	}
	if len(provider.lastFilters.ContentTypes) != 1 {
		t.Errorf("len(provider.lastFilters.ContentTypes) = %d, want 1", len(provider.lastFilters.ContentTypes))
	}
	if provider.lastFilters.Custom["team"] != "engineering" {
		t.Errorf("provider.lastFilters.Custom[team] = %v, want engineering", provider.lastFilters.Custom["team"])
	}
}

// --- Interface compliance tests ---

func TestDocumentIDFilterFactory_ImplementsStageFactory(t *testing.T) {
	var _ pipelineport.StageFactory = (*DocumentIDFilterFactory)(nil)
}

func TestDocumentIDFilterStage_ImplementsStage(t *testing.T) {
	var _ pipelineport.Stage = (*DocumentIDFilterStage)(nil)
}
