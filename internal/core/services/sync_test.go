package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// Test helper to create SyncOrchestrator with mocks
func createTestSyncOrchestrator(t *testing.T) (
	*SyncOrchestrator,
	*mocks.MockSourceStore,
	*mocks.MockDocumentStore,
	*mocks.MockSyncStateStore,
	*mocks.MockSearchEngine,
	*mockConnectorFactory,
) {
	t.Helper()

	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	// Mock indexing executor returns a synthetic output so the sync
	// orchestrator can advance — the chunk-level OpenSearch path was
	// removed, but these tests only need IndexingOutput to be non-nil.
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return &pipeline.IndexingOutput{
				DocumentID: input.DocumentID,
				ChunkIDs:   []string{input.DocumentID + "-chunk-0"},
			}, nil
		},
	}
	capabilitySet := pipeline.NewCapabilitySet()

	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    capabilitySet,
	})

	return orchestrator, sourceStore, documentStore, syncStore, searchEngine, connectorFactory
}

// mockConnectorFactory wraps mocks.MockConnectorFactory to fix interface compatibility
type mockConnectorFactory struct {
	connector *mocks.MockConnector
	createErr error
}

func newMockConnectorFactory() *mockConnectorFactory {
	return &mockConnectorFactory{
		connector: mocks.NewMockConnector(),
	}
}

func (m *mockConnectorFactory) Register(builder driven.ConnectorBuilder) {}

func (m *mockConnectorFactory) Create(ctx context.Context, source *domain.Source, containerID string) (driven.Connector, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.connector, nil
}

func (m *mockConnectorFactory) SupportedTypes() []domain.ProviderType {
	return []domain.ProviderType{domain.ProviderTypeGitHub}
}

func (m *mockConnectorFactory) GetBuilder(providerType domain.ProviderType) (driven.ConnectorBuilder, error) {
	return nil, nil
}

func (m *mockConnectorFactory) SupportsOAuth(providerType domain.ProviderType) bool {
	return false
}

func (m *mockConnectorFactory) GetOAuthConfig(providerType domain.ProviderType) *driven.OAuthConfig {
	return nil
}

// TestNewSyncOrchestrator tests basic orchestrator creation
func TestNewSyncOrchestrator(t *testing.T) {
	orchestrator, _, _, _, _, _ := createTestSyncOrchestrator(t)
	if orchestrator == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orchestrator.logger == nil {
		t.Error("expected non-nil logger")
	}
}

// TestNewSyncOrchestrator_NilLogger tests that a default logger is created when nil is provided
func TestNewSyncOrchestrator_NilLogger(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	executor := &mockIndexingExecutor{}
	capabilitySet := pipeline.NewCapabilitySet()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		Logger:           nil, // Explicitly nil
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    capabilitySet,
	})

	if orchestrator == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orchestrator.logger == nil {
		t.Fatal("expected non-nil logger even when not provided")
	}
}

// TestSyncSource_SourceNotFound tests error when source doesn't exist
func TestSyncSource_SourceNotFound(t *testing.T) {
	orchestrator, _, _, syncStore, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	result, err := orchestrator.SyncSource(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}

	// Verify sync state was updated to failed
	state, _ := syncStore.Get(ctx, "non-existent")
	if state != nil && state.Status != domain.SyncStatusFailed {
		t.Error("expected sync state to be failed")
	}
}

// TestSyncSource_DisabledSource tests that disabled sources fail sync
func TestSyncSource_DisabledSource(t *testing.T) {
	orchestrator, sourceStore, _, syncStore, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create disabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: false,
	}
	_ = sourceStore.Save(ctx, source)

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err == nil {
		t.Fatal("expected error for disabled source")
	}
	if result.Success {
		t.Error("expected Success=false for disabled source")
	}

	// Verify sync state was updated to failed
	state, _ := syncStore.Get(ctx, "source-1")
	if state != nil && state.Status != domain.SyncStatusFailed {
		t.Error("expected sync state to be failed")
	}
}

// TestSyncSource_ConnectorCreateFails tests error when connector creation fails
func TestSyncSource_ConnectorCreateFails(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Make connector creation fail
	connectorFactory.createErr = errors.New("connector creation failed")

	result, _ := orchestrator.SyncSource(ctx, "source-1")
	// With container-scoped sync, errors are captured in result.Error
	if result.Success {
		t.Error("expected Success=false")
	}
	if !containsString(result.Error, "failed to create connector") {
		t.Errorf("expected error to mention connector creation, got: %s", result.Error)
	}
}

// TestSyncSource_TestConnectionFails tests error when connection test fails
func TestSyncSource_TestConnectionFails(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Make test connection fail
	connectorFactory.connector.TestConnectionFn = func(ctx context.Context, source *domain.Source) error {
		return errors.New("connection test failed")
	}

	result, _ := orchestrator.SyncSource(ctx, "source-1")
	// With container-scoped sync, errors are captured in result.Error
	if result.Success {
		t.Error("expected Success=false")
	}
	if !containsString(result.Error, "connection test failed") {
		t.Errorf("expected error to mention connection test, got: %s", result.Error)
	}
}

// TestSyncSource_FetchChangesFails tests error when fetching changes fails
func TestSyncSource_FetchChangesFails(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Make fetch changes fail
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return nil, "", errors.New("fetch changes failed")
	}

	result, _ := orchestrator.SyncSource(ctx, "source-1")
	// With container-scoped sync, errors are captured in result.Error
	if result.Success {
		t.Error("expected Success=false")
	}
	if !containsString(result.Error, "failed to fetch changes") {
		t.Errorf("expected error to mention fetch changes, got: %s", result.Error)
	}
}

// TestSyncSource_ContextCancelled tests that context cancellation is handled
func TestSyncSource_ContextCancelled(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Cancel context before fetch returns
	callCount := 0
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		callCount++
		if callCount == 1 {
			// Cancel context after first call
			cancel()
			// Return a change to trigger another loop iteration
			return []*domain.Change{
				{ExternalID: "doc-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "doc-1"}},
			}, "cursor-1", nil
		}
		return nil, "", nil
	}

	result, _ := orchestrator.SyncSource(ctx, "source-1")
	// With container-scoped sync, context cancellation is captured in result.Error
	if result.Success {
		t.Error("expected Success=false")
	}
	// The error should mention context cancellation
	if !containsString(result.Error, "context canceled") && !containsString(result.Error, "context cancelled") {
		t.Errorf("expected error to mention context cancellation, got: %s", result.Error)
	}
}

// TestSyncSource_Success_AddDocument tests successful document addition
func TestSyncSource_Success_AddDocument(t *testing.T) {
	orchestrator, sourceStore, documentStore, syncStore, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
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
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}
	if result.Stats.ChunksIndexed != 1 {
		t.Errorf("expected 1 chunk indexed, got %d", result.Stats.ChunksIndexed)
	}

	// Verify document was saved
	savedDoc, err := documentStore.GetByExternalID(ctx, "source-1", "ext-1")
	if err != nil {
		t.Fatalf("document not found: %v", err)
	}
	if savedDoc.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got '%s'", savedDoc.Title)
	}
	if savedDoc.SourceID != "source-1" {
		t.Errorf("expected source ID 'source-1', got '%s'", savedDoc.SourceID)
	}

	// Verify the indexing executor reported chunks (the chunk-level
	// search-engine count is no longer maintained — chunks live in
	// pgvector now and this orchestrator-level test doesn't wire one up).
	if result.Stats.ChunksIndexed != 1 {
		t.Errorf("expected 1 chunk indexed, got %d", result.Stats.ChunksIndexed)
	}

	// Verify sync state was updated
	state, err := syncStore.Get(ctx, "source-1")
	if err != nil {
		t.Fatalf("sync state not found: %v", err)
	}
	if state.Status != domain.SyncStatusCompleted {
		t.Errorf("expected status completed, got %s", state.Status)
	}
	if state.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added in state, got %d", state.Stats.DocumentsAdded)
	}
}

// TestSyncSource_Success_UpdateDocument tests successful document update
func TestSyncSource_Success_UpdateDocument(t *testing.T) {
	orchestrator, sourceStore, documentStore, syncStore, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	// Pre-create existing document
	existingDoc := &domain.Document{
		ID:         "existing-doc-id",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "Old Title",
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	_ = documentStore.Save(ctx, existingDoc)

	// Setup connector to return modified document
	updatedDoc := &domain.Document{
		ExternalID: "ext-1",
		Title:      "New Title",
		MimeType:   "text/plain",
	}
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeModified,
					Document:   updatedDoc,
					Content:    "Updated content",
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
	if result.Stats.DocumentsUpdated != 1 {
		t.Errorf("expected 1 document updated, got %d", result.Stats.DocumentsUpdated)
	}
	if result.Stats.DocumentsAdded != 0 {
		t.Errorf("expected 0 documents added, got %d", result.Stats.DocumentsAdded)
	}

	// Verify document was updated but ID preserved
	savedDoc, _ := documentStore.GetByExternalID(ctx, "source-1", "ext-1")
	if savedDoc.ID != "existing-doc-id" {
		t.Error("expected original document ID to be preserved")
	}
	if savedDoc.Title != "New Title" {
		t.Errorf("expected title 'New Title', got '%s'", savedDoc.Title)
	}
	if savedDoc.CreatedAt != existingDoc.CreatedAt {
		t.Error("expected CreatedAt to be preserved")
	}

	// Verify sync state
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Stats.DocumentsUpdated != 1 {
		t.Errorf("expected 1 document updated in state, got %d", state.Stats.DocumentsUpdated)
	}
}

// TestSyncSource_Success_DeleteDocument tests successful document deletion
func TestSyncSource_Success_DeleteDocument(t *testing.T) {
	orchestrator, sourceStore, documentStore, syncStore, searchEngine, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	// Pre-create existing document
	existingDoc := &domain.Document{
		ID:         "doc-to-delete",
		SourceID:   "source-1",
		ExternalID: "ext-1",
		Title:      "To Delete",
	}
	_ = documentStore.Save(ctx, existingDoc)

	// Setup connector to return delete change
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeDeleted,
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
	if result.Stats.DocumentsDeleted != 1 {
		t.Errorf("expected 1 document deleted, got %d", result.Stats.DocumentsDeleted)
	}

	// Verify document was deleted
	_, err = documentStore.Get(ctx, "doc-to-delete")
	if err != domain.ErrNotFound {
		t.Error("expected document to be deleted")
	}

	// Verify search engine was updated
	count, _ := searchEngine.Count(ctx)
	if count != 0 {
		t.Error("expected chunks to be deleted from search engine")
	}

	// Verify sync state
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Stats.DocumentsDeleted != 1 {
		t.Errorf("expected 1 document deleted in state, got %d", state.Stats.DocumentsDeleted)
	}
}

// TestSyncSource_DeleteNonExistentDocument tests that deleting non-existent documents doesn't error
func TestSyncSource_DeleteNonExistentDocument(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Setup connector to delete non-existent document
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "non-existent",
					Type:       domain.ChangeTypeDeleted,
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
		t.Error("expected Success=true even when deleting non-existent document")
	}
	// Stats should not increment for non-existent documents
	if result.Stats.DocumentsDeleted != 0 {
		t.Errorf("expected 0 documents deleted, got %d", result.Stats.DocumentsDeleted)
	}
}

// TestSyncSource_Pagination tests handling of paginated results
func TestSyncSource_Pagination(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	// Setup connector to return documents across multiple pages
	pageNum := 0
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		pageNum++
		if pageNum == 1 {
			return []*domain.Change{
				{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1", Title: "Doc 1"}, Content: "Content 1"},
			}, "page2", nil
		}
		if pageNum == 2 && cursor == "page2" {
			return []*domain.Change{
				{ExternalID: "ext-2", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-2", Title: "Doc 2"}, Content: "Content 2"},
			}, "page3", nil
		}
		if pageNum == 3 && cursor == "page3" {
			return []*domain.Change{
				{ExternalID: "ext-3", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-3", Title: "Doc 3"}, Content: "Content 3"},
			}, "", nil // Last page
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stats.DocumentsAdded != 3 {
		t.Errorf("expected 3 documents added, got %d", result.Stats.DocumentsAdded)
	}

	count, _ := documentStore.CountBySource(ctx, "source-1")
	if count != 3 {
		t.Errorf("expected 3 documents in store, got %d", count)
	}
}

// TestSyncSource_PaginationStopsOnEmptyResults tests that pagination stops on empty results
func TestSyncSource_PaginationStopsOnEmptyResults(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	callCount := 0
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		callCount++
		if callCount == 1 {
			return []*domain.Change{
				{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1"}},
			}, "cursor-1", nil
		}
		// Return empty results - should stop pagination
		return []*domain.Change{}, "cursor-2", nil
	}

	_, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls to FetchChanges, got %d", callCount)
	}

	count, _ := documentStore.CountBySource(ctx, "source-1")
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
}

// TestSyncSource_PaginationStopsOnSameCursor tests that pagination stops when cursor doesn't advance
func TestSyncSource_PaginationStopsOnSameCursor(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	callCount := 0
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		callCount++
		// Return same cursor - should stop to prevent infinite loop
		return []*domain.Change{
			{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1"}},
		}, cursor, nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call to FetchChanges (stopped due to same cursor), got %d", callCount)
	}

	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}
}

// TestSyncSource_ProcessChangeError_Continues tests that sync continues after processing errors
func TestSyncSource_ProcessChangeError_Continues(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	// First document has nil Document (will error), second is valid
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return []*domain.Change{
			{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: nil, Content: "Content"},                                                     // Will fail
			{ExternalID: "ext-2", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-2", Title: "Valid"}, Content: "Content 2"}, // Should succeed
		}, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stats.Errors != 1 {
		t.Errorf("expected 1 error, got %d", result.Stats.Errors)
	}
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}

	count, _ := documentStore.CountBySource(ctx, "source-1")
	if count != 1 {
		t.Errorf("expected 1 document in store (only the valid one), got %d", count)
	}
}

// TestSyncSource_UnknownChangeType tests error handling for unknown change types
func TestSyncSource_UnknownChangeType(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create enabled source
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return []*domain.Change{
			{ExternalID: "ext-1", Type: "unknown_type", Document: &domain.Document{ExternalID: "ext-1"}},
		}, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stats.Errors != 1 {
		t.Errorf("expected 1 error for unknown change type, got %d", result.Stats.Errors)
	}
}

// TestSyncSource_MultipleChunks tests that multiple chunks are created and indexed
func TestSyncSource_MultipleChunks(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Mock indexing executor returns synthetic 3-chunk output.
	executor := orchestrator.indexingExecutor.(*mockIndexingExecutor)
	executor.executeFn = func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
		return &pipeline.IndexingOutput{
			DocumentID: input.DocumentID,
			ChunkIDs:   []string{"chunk-1", "chunk-2", "chunk-3"},
		}, nil
	}

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1"}, Content: "Test content for chunking"},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.ChunksIndexed != 3 {
		t.Errorf("expected 3 chunks indexed, got %d", result.Stats.ChunksIndexed)
	}
}

// TestSyncSource_NormaliserApplied tests that content normalisation is applied
func TestSyncSource_NormaliserApplied(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Mock normaliser to uppercase content
	normaliser := mocks.NewMockNormaliser()
	normaliser.NormaliseFn = func(content string, mimeType string) string {
		return "NORMALIZED: " + content
	}
	// We can't type assert because normaliserReg is private interface,
	// so we need to work with the existing mock
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	normaliserRegistry.SetNormaliser(normaliser)
	orchestrator.normaliserReg = normaliserRegistry

	// Track what content was passed to the pipeline executor
	var processedContent string
	executor := orchestrator.indexingExecutor.(*mockIndexingExecutor)
	executor.executeFn = func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
		processedContent = input.Content
		return &pipeline.IndexingOutput{
			DocumentID: input.DocumentID,
			ChunkIDs:   []string{input.DocumentID + "-chunk-0"},
		}, nil
	}

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-1", MimeType: "text/plain"},
					Content:    "original content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	_, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if processedContent != "NORMALIZED: original content" {
		t.Errorf("expected normalised content, got: %s", processedContent)
	}
}

// TestSyncSource_NilSearchEngine tests that sync works without search engine
func TestSyncSource_NilSearchEngine(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	executor := &mockIndexingExecutor{}
	capabilitySet := pipeline.NewCapabilitySet()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

	// Create orchestrator without search engine
	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		SyncStore:        syncStore,
		SearchEngine:     nil, // No search engine
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    capabilitySet,
	})

	ctx := context.Background()
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-1", Enabled: true})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return []*domain.Change{
			{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1"}, Content: "Content"},
		}, "", nil
	}

	// Should not panic without search engine
	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success even without search engine")
	}
}

// TestSyncSource_CursorPersisted tests that cursor is persisted after successful sync
func TestSyncSource_CursorPersisted(t *testing.T) {
	orchestrator, sourceStore, _, syncStore, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{ExternalID: "ext-1", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "ext-1"}},
			}, "final-cursor", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Cursor != "final-cursor" {
		t.Errorf("expected cursor 'final-cursor', got '%s'", result.Cursor)
	}

	// Verify cursor was saved in sync state
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Cursor != "final-cursor" {
		t.Errorf("expected cursor 'final-cursor' in state, got '%s'", state.Cursor)
	}
}

// TestSyncSource_SyncStateProgression tests sync state transitions
func TestSyncSource_SyncStateProgression(t *testing.T) {
	orchestrator, sourceStore, _, syncStore, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Pre-create sync state
	_ = syncStore.Save(ctx, &domain.SyncState{
		SourceID: "source-1",
		Status:   domain.SyncStatusIdle,
	})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		// Verify state was set to running
		state, _ := syncStore.Get(ctx, "source-1")
		if state.Status != domain.SyncStatusRunning {
			t.Errorf("expected status running during sync, got %s", state.Status)
		}
		return []*domain.Change{}, "", nil
	}

	_, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify final state is completed
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Status != domain.SyncStatusCompleted {
		t.Errorf("expected status completed after sync, got %s", state.Status)
	}
	if state.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if state.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if state.LastSyncAt == nil {
		t.Error("expected LastSyncAt to be set")
	}
}

// TestSyncAll_NoSources tests SyncAll with no sources
func TestSyncAll_NoSources(t *testing.T) {
	orchestrator, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	results, err := orchestrator.SyncAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestSyncAll_SkipsDisabledSources tests that disabled sources are skipped
func TestSyncAll_SkipsDisabledSources(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create disabled source
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-1", Name: "Disabled", Enabled: false})
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-2", Name: "Disabled 2", Enabled: false})

	results, err := orchestrator.SyncAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (disabled sources skipped), got %d", len(results))
	}
}

// TestSyncAll_MultipleSources tests syncing multiple enabled sources
func TestSyncAll_MultipleSources(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create two enabled sources
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-1", Name: "Source 1", Enabled: true})
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-2", Name: "Source 2", Enabled: true})

	// Setup connector to return different docs per source
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return []*domain.Change{
			{ExternalID: source.ID + "-doc", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: source.ID + "-doc"}, Content: "Content"},
		}, "", nil
	}

	results, err := orchestrator.SyncAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// All should be successful
	for _, r := range results {
		if !r.Success {
			t.Errorf("expected all syncs to succeed, but %s failed: %s", r.SourceID, r.Error)
		}
	}
}

// TestSyncAll_MixedEnabledDisabled tests that only enabled sources are synced
func TestSyncAll_MixedEnabledDisabled(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-1", Enabled: true})
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-2", Enabled: false})
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-3", Enabled: true})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		return []*domain.Change{}, "", nil
	}

	results, err := orchestrator.SyncAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (only enabled sources), got %d", len(results))
	}
}

// TestSyncAll_ListSourcesError tests error when listing sources fails
func TestSyncAll_ListSourcesError(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create a custom mock that returns an error on List
	mockStore := &mockSourceStoreWithError{
		MockSourceStore: sourceStore,
		listErr:         errors.New("list failed"),
	}
	orchestrator.sourceStore = mockStore

	_, err := orchestrator.SyncAll(ctx)
	if err == nil {
		t.Fatal("expected error when list sources fails")
	}
	if !containsString(err.Error(), "failed to list sources") {
		t.Errorf("expected error to mention list sources, got: %v", err)
	}
}

// mockSourceStoreWithError wraps MockSourceStore to inject errors
type mockSourceStoreWithError struct {
	*mocks.MockSourceStore
	listErr error
}

func (m *mockSourceStoreWithError) List(ctx context.Context) ([]*domain.Source, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.MockSourceStore.List(ctx)
}

// TestSyncAll_PartialFailure tests that SyncAll continues after individual source failures
func TestSyncAll_PartialFailure(t *testing.T) {
	orchestrator, sourceStore, _, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Create two sources
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-1", Enabled: true})
	_ = sourceStore.Save(ctx, &domain.Source{ID: "source-2", Enabled: true})

	// First source succeeds, second fails
	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if source.ID == "source-2" {
			return nil, "", errors.New("fetch failed")
		}
		return []*domain.Change{
			{ExternalID: "doc", Type: domain.ChangeTypeAdded, Document: &domain.Document{ExternalID: "doc"}, Content: "Content"},
		}, "", nil
	}

	results, err := orchestrator.SyncAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Count successes and failures
	var successes, failures int
	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Errorf("expected 1 success and 1 failure, got %d successes and %d failures", successes, failures)
	}

	// Verify failed result has error message
	for _, r := range results {
		if !r.Success && r.Error == "" {
			t.Error("expected error message in failed result")
		}
	}
}

// TestProcessChange_NilDocument tests processAddOrUpdate with nil document
func TestProcessChange_NilDocument(t *testing.T) {
	orchestrator, sourceStore, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	change := &domain.Change{
		Type:       domain.ChangeTypeAdded,
		ExternalID: "ext-1",
		Document:   nil, // Nil document
		Content:    "Content",
	}

	stats := &domain.SyncStats{}
	err := orchestrator.processChange(ctx, source, change, stats)
	if err == nil {
		t.Fatal("expected error for nil document")
	}
	if !containsString(err.Error(), "document is nil") {
		t.Errorf("expected error to mention nil document, got: %v", err)
	}

	// Stats should not increment
	if stats.DocumentsAdded != 0 {
		t.Error("expected stats to not increment on error")
	}
}

// TestProcessAddOrUpdate_DocumentFieldsSet tests that document fields are properly set
func TestProcessAddOrUpdate_DocumentFieldsSet(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "source-1"}
	_ = sourceStore.Save(ctx, source)

	change := &domain.Change{
		Type:       domain.ChangeTypeAdded,
		ExternalID: "ext-1",
		Document: &domain.Document{
			Title: "Test",
		},
		Content: "Content",
	}

	stats := &domain.SyncStats{}
	err := orchestrator.processAddOrUpdate(ctx, source, change, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify document fields were set
	doc, _ := documentStore.GetByExternalID(ctx, "source-1", "ext-1")
	if doc.SourceID != "source-1" {
		t.Error("expected SourceID to be set")
	}
	if doc.ExternalID != "ext-1" {
		t.Error("expected ExternalID to be set")
	}
	if doc.ID == "" {
		t.Error("expected ID to be generated")
	}
	if doc.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if doc.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if doc.IndexedAt.IsZero() {
		t.Error("expected IndexedAt to be set")
	}
}

// TestFailSync tests that failSync properly updates sync state
func TestFailSync(t *testing.T) {
	orchestrator, _, _, syncStore, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	// Pre-create sync state
	_ = syncStore.Save(ctx, &domain.SyncState{
		SourceID: "source-1",
		Status:   domain.SyncStatusRunning,
	})

	testErr := errors.New("sync failed")
	startTime := time.Now().Add(-5 * time.Second)

	result, err := orchestrator.failSync(ctx, "source-1", startTime, testErr)

	if err != testErr {
		t.Errorf("expected error to be returned, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Error != "sync failed" {
		t.Errorf("expected error message, got: %s", result.Error)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}

	// Verify sync state was updated
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Status != domain.SyncStatusFailed {
		t.Errorf("expected status failed, got %s", state.Status)
	}
	if state.Error != "sync failed" {
		t.Errorf("expected error message in state, got: %s", state.Error)
	}
	if state.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

// TestFailSync_NoExistingSyncState tests failSync when no sync state exists
func TestFailSync_NoExistingSyncState(t *testing.T) {
	orchestrator, _, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()

	testErr := errors.New("sync failed")
	startTime := time.Now()

	result, err := orchestrator.failSync(ctx, "source-1", startTime, testErr)

	// Should still return result even if sync state doesn't exist
	if err != testErr {
		t.Errorf("expected error to be returned")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Error("expected Success=false")
	}
}

// TestSyncSource_SearchEngineDeleteError tests that deletion continues even if search engine fails
func TestSyncSource_SearchEngineDeleteError(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:      "source-1",
		Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Pre-create document
	existingDoc := &domain.Document{
		ID:         "doc-1",
		SourceID:   "source-1",
		ExternalID: "ext-1",
	}
	_ = documentStore.Save(ctx, existingDoc)

	// Mock search engine that errors on delete
	searchEngine := &mockSearchEngineWithError{
		MockSearchEngine: orchestrator.searchEngine.(*mocks.MockSearchEngine),
		deleteErr:        errors.New("search engine delete failed"),
	}
	orchestrator.searchEngine = searchEngine

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{ExternalID: "ext-1", Type: domain.ChangeTypeDeleted},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still succeed and delete from document store
	if !result.Success {
		t.Error("expected success even with search engine error")
	}
	if result.Stats.DocumentsDeleted != 1 {
		t.Errorf("expected 1 document deleted, got %d", result.Stats.DocumentsDeleted)
	}

	// Document should be deleted from document store
	_, err = documentStore.Get(ctx, "doc-1")
	if err != domain.ErrNotFound {
		t.Error("expected document to be deleted from document store")
	}
}

// mockSearchEngineWithError wraps MockSearchEngine to inject errors
type mockSearchEngineWithError struct {
	*mocks.MockSearchEngine
	deleteErr error
}

func (m *mockSearchEngineWithError) DeleteByDocument(ctx context.Context, documentID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return m.MockSearchEngine.DeleteByDocument(ctx, documentID)
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Phase-1 reconciliation (ReconciliationScopes + Inventory) ------------

// TestReconcile_NoScopes verifies the happy path for connectors that
// opt out of snapshot reconciliation: nothing in phase 1 runs, and phase 2
// proceeds normally. This is the OneDrive shape (native @removed tombstones).
func TestReconcile_NoScopes(t *testing.T) {
	orch, sourceStore, documentStore, _, _, cf := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "src", ProviderType: domain.ProviderTypeGitHub, Enabled: true}
	_ = sourceStore.Save(ctx, source)

	// Pre-existing doc that the connector no longer knows about. If phase-1
	// ran without scopes declared, this would (incorrectly) get deleted.
	_ = documentStore.Save(ctx, &domain.Document{
		ID: "d1", SourceID: "src", ExternalID: "file-orphan.md",
	})

	cf.connector.ReconciliationScopesFn = func() []string { return nil }
	cf.connector.InventoryFn = func(ctx context.Context, _ *domain.Source, _ string) ([]string, error) {
		t.Fatal("Inventory must not be called when ReconciliationScopes is empty")
		return nil, nil
	}
	cf.connector.FetchChangesFn = func(context.Context, *domain.Source, string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "src")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if result.Stats.DocumentsDeleted != 0 {
		t.Errorf("no scopes declared, but %d delete(s) fired", result.Stats.DocumentsDeleted)
	}
	// Doc must still be present.
	if _, err := documentStore.GetByExternalID(ctx, "src", "file-orphan.md"); err != nil {
		t.Errorf("pre-existing doc was deleted despite no scopes: %v", err)
	}
}

// TestReconcile_DeletesOrphans is the core positive test: the connector's
// inventory omits a stored external ID, and the orchestrator emits a delete
// through the normal processChange path.
func TestReconcile_DeletesOrphans(t *testing.T) {
	orch, sourceStore, documentStore, _, _, cf := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "src", ProviderType: domain.ProviderTypeGitHub, Enabled: true}
	_ = sourceStore.Save(ctx, source)

	// Two stored docs under the scope; only one is still present upstream.
	_ = documentStore.Save(ctx, &domain.Document{ID: "d-keep", SourceID: "src", ExternalID: "file-keep.md"})
	_ = documentStore.Save(ctx, &domain.Document{ID: "d-gone", SourceID: "src", ExternalID: "file-gone.md"})
	// A doc under a different prefix — must not be touched.
	_ = documentStore.Save(ctx, &domain.Document{ID: "d-issue", SourceID: "src", ExternalID: "issue-1"})

	cf.connector.ReconciliationScopesFn = func() []string { return []string{"file-"} }
	cf.connector.InventoryFn = func(_ context.Context, _ *domain.Source, scope string) ([]string, error) {
		if scope != "file-" {
			t.Errorf("unexpected scope %q", scope)
		}
		return []string{"file-keep.md"}, nil
	}
	cf.connector.FetchChangesFn = func(context.Context, *domain.Source, string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "src")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if result.Stats.DocumentsDeleted != 1 {
		t.Errorf("want 1 delete, got %d", result.Stats.DocumentsDeleted)
	}
	if _, err := documentStore.GetByExternalID(ctx, "src", "file-gone.md"); err == nil {
		t.Error("file-gone.md should have been deleted")
	}
	if _, err := documentStore.GetByExternalID(ctx, "src", "file-keep.md"); err != nil {
		t.Errorf("file-keep.md should be untouched: %v", err)
	}
	if _, err := documentStore.GetByExternalID(ctx, "src", "issue-1"); err != nil {
		t.Errorf("issue-1 (different prefix) should be untouched: %v", err)
	}
}

// TestReconcile_InventoryErrorIsTolerated asserts the best-effort contract:
// an Inventory failure logs and moves on. Phase 2 still runs; no
// unrelated docs get deleted.
func TestReconcile_InventoryErrorIsTolerated(t *testing.T) {
	orch, sourceStore, documentStore, _, _, cf := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "src", ProviderType: domain.ProviderTypeGitHub, Enabled: true}
	_ = sourceStore.Save(ctx, source)
	_ = documentStore.Save(ctx, &domain.Document{ID: "d1", SourceID: "src", ExternalID: "file-a.md"})

	cf.connector.ReconciliationScopesFn = func() []string { return []string{"file-"} }
	cf.connector.InventoryFn = func(context.Context, *domain.Source, string) ([]string, error) {
		return nil, errors.New("rate limited")
	}
	cf.connector.FetchChangesFn = func(context.Context, *domain.Source, string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "src")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if result.Stats.DocumentsDeleted != 0 {
		t.Errorf("Inventory error must not drive deletions, got %d", result.Stats.DocumentsDeleted)
	}
	if _, err := documentStore.GetByExternalID(ctx, "src", "file-a.md"); err != nil {
		t.Errorf("doc was deleted despite Inventory error: %v", err)
	}
}

// TestReconcile_EmptyInventoryStillDeletes — when the upstream legitimately
// holds nothing under a scope, the connector returns an empty inventory and
// every stored ID under that scope is an orphan. The orchestrator logs loudly
// (see reconcileDeletions) but must still perform the deletes; refusing
// would re-introduce the finding #100 bug it was built to fix.
func TestReconcile_EmptyInventoryStillDeletes(t *testing.T) {
	orch, sourceStore, documentStore, _, _, cf := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "src", ProviderType: domain.ProviderTypeGitHub, Enabled: true}
	_ = sourceStore.Save(ctx, source)
	_ = documentStore.Save(ctx, &domain.Document{ID: "d1", SourceID: "src", ExternalID: "page-a"})
	_ = documentStore.Save(ctx, &domain.Document{ID: "d2", SourceID: "src", ExternalID: "page-b"})

	cf.connector.ReconciliationScopesFn = func() []string { return []string{"page-"} }
	cf.connector.InventoryFn = func(context.Context, *domain.Source, string) ([]string, error) {
		return []string{}, nil
	}
	cf.connector.FetchChangesFn = func(context.Context, *domain.Source, string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "src")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if result.Stats.DocumentsDeleted != 2 {
		t.Errorf("want 2 deletes, got %d", result.Stats.DocumentsDeleted)
	}
}

// TestReconcile_SkipsScopeWithNoStoredIDs — if nothing is indexed under a
// scope, Inventory must not be called at all. This avoids paying the API
// cost of enumeration when there's nothing to reconcile against.
func TestReconcile_SkipsScopeWithNoStoredIDs(t *testing.T) {
	orch, sourceStore, _, _, _, cf := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{ID: "src", ProviderType: domain.ProviderTypeGitHub, Enabled: true}
	_ = sourceStore.Save(ctx, source)

	cf.connector.ReconciliationScopesFn = func() []string { return []string{"file-", "issue-"} }
	cf.connector.InventoryFn = func(context.Context, *domain.Source, string) ([]string, error) {
		t.Fatal("Inventory must not be called when no stored IDs match the scope")
		return nil, nil
	}
	cf.connector.FetchChangesFn = func(context.Context, *domain.Source, string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	if _, err := orch.SyncSource(ctx, "src"); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
}

// TestSyncSource_SkipsWhenLockHeld — when a per-source lock is configured
// and another caller already holds it, SyncSource must return Skipped=true
// (Success=true so the worker acks the task) rather than racing the
// in-flight sync. Reproduces the bug from the production logs where a
// sync_container task and a sync_source task for the same source ran two
// seconds apart and both cleaned chunks while the other was indexing.
func TestSyncSource_SkipsWhenLockHeld(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	connectorFactory := newMockConnectorFactory()
	services := &runtime.Services{}
	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, in *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return &pipeline.IndexingOutput{DocumentID: in.DocumentID}, nil
		},
	}
	lock := mocks.NewMockDistributedLock()
	orch := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		Services:         services,
		IndexingExecutor: executor,
		CapabilitySet:    pipeline.NewCapabilitySet(),
		Lock:             lock,
	})

	ctx := context.Background()
	source := &domain.Source{ID: "source-1", Name: "S", Enabled: true}
	_ = sourceStore.Save(ctx, source)

	// Simulate another worker already running a sync for this source.
	lock.SetLockHeld("sync:source:source-1", time.Hour)

	result, err := orch.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if !result.Skipped {
		t.Errorf("want Skipped=true when lock is held, got %+v", result)
	}
	if !result.Success {
		t.Errorf("Skipped runs must be Success=true so the worker acks the task")
	}

	// Same lock, same key → SyncContainer must also skip.
	cResult, err := orch.SyncContainer(ctx, "source-1", "container-1")
	if err != nil {
		t.Fatalf("SyncContainer: %v", err)
	}
	if !cResult.Skipped {
		t.Errorf("SyncContainer must also skip when source lock is held: %+v", cResult)
	}
}

// TestSyncSource_NoLockBackend — when no lock is configured, sync must
// proceed unchanged (single-instance mode).
func TestSyncSource_NoLockBackend(t *testing.T) {
	orch, sourceStore, _, _, _, _ := createTestSyncOrchestrator(t)
	ctx := context.Background()
	source := &domain.Source{ID: "source-1", Name: "S", Enabled: true}
	_ = sourceStore.Save(ctx, source)

	result, err := orch.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if result.Skipped {
		t.Error("must not skip when no lock backend is configured")
	}
}
