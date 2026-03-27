package services

import (
	"context"
	"errors"
	"testing"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven/mocks"
	"github.com/custodia-labs/sercha-core/internal/runtime"
)

// mockIndexingExecutor is a mock implementation of IndexingExecutor for testing
type mockIndexingExecutor struct {
	executeFn      func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error)
	executeBatchFn func(ctx context.Context, pctx *pipeline.IndexingContext, inputs []*pipeline.IndexingInput) ([]*pipeline.IndexingOutput, error)
	executeCount   int
}

func (m *mockIndexingExecutor) Execute(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
	m.executeCount++
	if m.executeFn != nil {
		return m.executeFn(ctx, pctx, input)
	}
	// Default implementation returns success
	return &pipeline.IndexingOutput{
		DocumentID: input.DocumentID,
		ChunkIDs:   []string{input.DocumentID + "-chunk-0"},
		Manifest:   nil,
	}, nil
}

func (m *mockIndexingExecutor) ExecuteBatch(ctx context.Context, pctx *pipeline.IndexingContext, inputs []*pipeline.IndexingInput) ([]*pipeline.IndexingOutput, error) {
	if m.executeBatchFn != nil {
		return m.executeBatchFn(ctx, pctx, inputs)
	}
	// Default implementation
	outputs := make([]*pipeline.IndexingOutput, len(inputs))
	for i, input := range inputs {
		outputs[i] = &pipeline.IndexingOutput{
			DocumentID: input.DocumentID,
			ChunkIDs:   []string{input.DocumentID + "-chunk-0"},
		}
	}
	return outputs, nil
}

// TestSyncOrchestrator_WithPipelineExecutor tests that SyncOrchestrator uses pipeline executor when provided
func TestSyncOrchestrator_WithPipelineExecutor(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	chunkStore := mocks.NewMockChunkStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	legacyPipeline := mocks.NewMockPostProcessorPipeline()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	// Create mock executor
	executor := &mockIndexingExecutor{}
	capabilitySet := pipeline.NewCapabilitySet()

	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		ChunkStore:       chunkStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		LegacyPipeline:   legacyPipeline,
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    capabilitySet,
	})

	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Setup connector to return one document
	doc := &domain.Document{
		ID:         "doc-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
		MimeType:   "text/plain",
	}
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeAdded,
					Document:   doc,
					Content:    "Test content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}

	// Verify pipeline executor was called
	if executor.executeCount != 1 {
		t.Errorf("expected pipeline executor to be called once, got %d calls", executor.executeCount)
	}

	// Verify document was saved
	savedDoc, err := documentStore.GetByExternalID(ctx, "source-1", "ext-1")
	if err != nil {
		t.Fatalf("document not found: %v", err)
	}
	if savedDoc.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got '%s'", savedDoc.Title)
	}

	// Verify stats
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}
	if result.Stats.ChunksIndexed != 1 {
		t.Errorf("expected 1 chunk indexed, got %d", result.Stats.ChunksIndexed)
	}
}

// TestSyncOrchestrator_PipelineExecutorFallback tests fallback to legacy pipeline when executor fails
func TestSyncOrchestrator_PipelineExecutorFallback(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	chunkStore := mocks.NewMockChunkStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	legacyPipeline := mocks.NewMockPostProcessorPipeline()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	// Create mock executor that fails
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return nil, errors.New("pipeline execution failed")
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		ChunkStore:       chunkStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		LegacyPipeline:   legacyPipeline,
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    capabilitySet,
	})

	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Setup connector to return one document
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-1", Title: "Test Doc"},
					Content:    "Test content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true (should fall back to legacy)")
	}

	// Verify pipeline executor was attempted
	if executor.executeCount != 1 {
		t.Errorf("expected pipeline executor to be attempted once, got %d calls", executor.executeCount)
	}

	// Verify document was still processed via legacy pipeline
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added via fallback, got %d", result.Stats.DocumentsAdded)
	}
}

// TestSyncOrchestrator_NilExecutorUsesLegacy tests that nil executor uses legacy pipeline
func TestSyncOrchestrator_NilExecutorUsesLegacy(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	chunkStore := mocks.NewMockChunkStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	legacyPipeline := mocks.NewMockPostProcessorPipeline()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	// Create orchestrator with nil executor
	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		ChunkStore:       chunkStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		LegacyPipeline:   legacyPipeline,
		Services:         services,
		IndexingExecutor: nil, // No executor - should use legacy
		CapabilitySet:    nil,
	})

	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Setup connector to return one document
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-1", Title: "Test Doc"},
					Content:    "Test content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true with legacy pipeline")
	}

	// Verify document was processed
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added via legacy, got %d", result.Stats.DocumentsAdded)
	}
}

// TestProcessWithPipeline_Success tests successful pipeline execution
func TestProcessWithPipeline_Success(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Add pipeline executor
	executor := &mockIndexingExecutor{}
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = pipeline.NewCapabilitySet()

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
		MimeType:   "text/plain",
		Path:       "/test/path",
		Metadata:   map[string]string{"key": "value"},
	}
	content := "Test content"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify executor was called
	if executor.executeCount != 1 {
		t.Errorf("expected executor to be called once, got %d", executor.executeCount)
	}

	// Verify document was saved
	savedDoc, err := documentStore.Get(ctx, "doc-1")
	if err != nil {
		t.Fatalf("document not saved: %v", err)
	}
	if savedDoc.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got '%s'", savedDoc.Title)
	}

	// Verify stats
	if stats.DocumentsAdded != 1 {
		t.Errorf("expected DocumentsAdded=1, got %d", stats.DocumentsAdded)
	}
	if stats.ChunksIndexed != 1 {
		t.Errorf("expected ChunksIndexed=1, got %d", stats.ChunksIndexed)
	}
}

// TestProcessWithPipeline_UpdateDocument tests pipeline execution for document updates
func TestProcessWithPipeline_UpdateDocument(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Add pipeline executor
	executor := &mockIndexingExecutor{}
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = pipeline.NewCapabilitySet()

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Updated Doc",
	}
	content := "Updated content"
	isUpdate := true
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify stats for update
	if stats.DocumentsUpdated != 1 {
		t.Errorf("expected DocumentsUpdated=1, got %d", stats.DocumentsUpdated)
	}
	if stats.DocumentsAdded != 0 {
		t.Errorf("expected DocumentsAdded=0 for update, got %d", stats.DocumentsAdded)
	}
}

// TestProcessWithPipeline_ExecutorError tests error handling when executor fails
func TestProcessWithPipeline_ExecutorError(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Add pipeline executor that fails
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return nil, errors.New("executor error")
		},
	}
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = pipeline.NewCapabilitySet()

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
	}
	content := "Test content"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err == nil {
		t.Fatal("expected error from pipeline executor")
	}
	if !containsString(err.Error(), "pipeline execution failed") {
		t.Errorf("expected error to mention pipeline execution, got: %v", err)
	}

	// Verify stats not incremented on error
	if stats.DocumentsAdded != 0 {
		t.Errorf("expected stats not to be incremented on error, got DocumentsAdded=%d", stats.DocumentsAdded)
	}
}

// TestProcessWithPipeline_MultipleChunks tests pipeline execution with multiple chunks
func TestProcessWithPipeline_MultipleChunks(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Add pipeline executor that returns multiple chunks
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return &pipeline.IndexingOutput{
				DocumentID: input.DocumentID,
				ChunkIDs:   []string{"chunk-1", "chunk-2", "chunk-3"},
				Manifest:   nil,
			}, nil
		},
	}
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = pipeline.NewCapabilitySet()

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
	}
	content := "Long test content that gets chunked into multiple pieces"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify multiple chunks were indexed
	if stats.ChunksIndexed != 3 {
		t.Errorf("expected ChunksIndexed=3, got %d", stats.ChunksIndexed)
	}
}

// TestProcessWithPipeline_MetadataConversion tests metadata conversion from map[string]string to map[string]any
func TestProcessWithPipeline_MetadataConversion(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Track what input was sent to executor
	var capturedInput *pipeline.IndexingInput
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			capturedInput = input
			return &pipeline.IndexingOutput{
				DocumentID: input.DocumentID,
				ChunkIDs:   []string{"chunk-1"},
			}, nil
		},
	}
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = pipeline.NewCapabilitySet()

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
		MimeType:   "text/plain",
		Path:       "/test",
		Metadata:   map[string]string{"author": "John", "type": "article"},
	}
	content := "Test content"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metadata was converted correctly
	if capturedInput == nil {
		t.Fatal("executor was not called")
	}
	if capturedInput.Metadata == nil {
		t.Fatal("metadata was not passed to executor")
	}
	if capturedInput.Metadata["author"] != "John" {
		t.Errorf("expected metadata author=John, got %v", capturedInput.Metadata["author"])
	}
	if capturedInput.Metadata["type"] != "article" {
		t.Errorf("expected metadata type=article, got %v", capturedInput.Metadata["type"])
	}

	// Verify all document fields were passed
	if capturedInput.DocumentID != "doc-1" {
		t.Errorf("expected DocumentID=doc-1, got %s", capturedInput.DocumentID)
	}
	if capturedInput.Title != "Test Doc" {
		t.Errorf("expected Title='Test Doc', got %s", capturedInput.Title)
	}
	if capturedInput.Content != "Test content" {
		t.Errorf("expected Content='Test content', got %s", capturedInput.Content)
	}
	if capturedInput.MimeType != "text/plain" {
		t.Errorf("expected MimeType='text/plain', got %s", capturedInput.MimeType)
	}
	if capturedInput.Path != "/test" {
		t.Errorf("expected Path='/test', got %s", capturedInput.Path)
	}
}

// TestProcessWithPipeline_ContextPassing tests that correct context is passed to executor
func TestProcessWithPipeline_ContextPassing(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	// Track what context was sent to executor
	var capturedContext *pipeline.IndexingContext
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			capturedContext = pctx
			return &pipeline.IndexingOutput{
				DocumentID: input.DocumentID,
				ChunkIDs:   []string{"chunk-1"},
			}, nil
		},
	}
	capSet := pipeline.NewCapabilitySet()
	orchestrator.indexingExecutor = executor
	orchestrator.capabilitySet = capSet

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
	}
	content := "Test content"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithPipeline(ctx, source, doc, content, isUpdate, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify context was passed correctly
	if capturedContext == nil {
		t.Fatal("executor was not called with context")
	}
	if capturedContext.PipelineID != "default-indexing" {
		t.Errorf("expected PipelineID='default-indexing', got %s", capturedContext.PipelineID)
	}
	if capturedContext.SourceID != "source-1" {
		t.Errorf("expected SourceID='source-1', got %s", capturedContext.SourceID)
	}
	if capturedContext.ConnectorID != "source-1" {
		t.Errorf("expected ConnectorID='source-1', got %s", capturedContext.ConnectorID)
	}
	if capturedContext.Capabilities != capSet {
		t.Error("expected capabilities to be passed through")
	}
}

// TestProcessWithLegacy_BackwardCompatibility tests that legacy pipeline still works
func TestProcessWithLegacy_BackwardCompatibility(t *testing.T) {
	orchestrator, sourceStore, documentStore, chunkStore, _, searchEngine, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	doc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Test Doc",
	}
	content := "Test content"
	isUpdate := false
	stats := &domain.SyncStats{}

	err := orchestrator.processWithLegacy(ctx, doc, content, isUpdate, stats, doc.CreatedAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify document was saved
	savedDoc, err := documentStore.Get(ctx, "doc-1")
	if err != nil {
		t.Fatalf("document not saved: %v", err)
	}
	if savedDoc.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got '%s'", savedDoc.Title)
	}

	// Verify chunks were saved
	chunks, _ := chunkStore.GetByDocument(ctx, "doc-1")
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	// Verify chunks were indexed
	count, _ := searchEngine.Count(ctx)
	if count != 1 {
		t.Errorf("expected 1 chunk in search engine, got %d", count)
	}

	// Verify stats
	if stats.DocumentsAdded != 1 {
		t.Errorf("expected DocumentsAdded=1, got %d", stats.DocumentsAdded)
	}
	if stats.ChunksIndexed != 1 {
		t.Errorf("expected ChunksIndexed=1, got %d", stats.ChunksIndexed)
	}
}
