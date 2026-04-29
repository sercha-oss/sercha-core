package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// spyDocumentIngestObserver is a test-local spy for driven.DocumentIngestObserver.
// It records invocations and the (source, doc) pair it received, and can be
// configured to return a sentinel error.
type spyDocumentIngestObserver struct {
	mu         sync.Mutex
	calls      int
	lastSource *domain.Source
	lastDoc    *domain.Document
	returnErr  error
}

func (s *spyDocumentIngestObserver) OnDocumentIngested(
	_ context.Context,
	source *domain.Source,
	doc *domain.Document,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastSource = source
	s.lastDoc = doc
	return s.returnErr
}

func (s *spyDocumentIngestObserver) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *spyDocumentIngestObserver) Last() (*domain.Source, *domain.Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSource, s.lastDoc
}

// mockDocumentStoreWithSaveError wraps MockDocumentStore to inject a Save error.
// MockDocumentStore has no built-in SaveErr hook, so we override Save here
// (same pattern as mockSearchEngineWithError in sync_test.go).
type mockDocumentStoreWithSaveError struct {
	*mocks.MockDocumentStore
	saveErr error
}

func (m *mockDocumentStoreWithSaveError) Save(ctx context.Context, doc *domain.Document) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	return m.MockDocumentStore.Save(ctx, doc)
}

// createTestSyncOrchestratorWithObserver builds a SyncOrchestrator wired with
// the supplied observer and (optional) logger. It mirrors createTestSyncOrchestrator
// in sync_test.go but exposes the observer + logger for the hook tests.
func createTestSyncOrchestratorWithObserver(
	t *testing.T,
	observer driven.DocumentIngestObserver,
	logger *slog.Logger,
) (
	*SyncOrchestrator,
	*mocks.MockSourceStore,
	*mocks.MockDocumentStore,
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
		SourceStore:            sourceStore,
		DocumentStore:          documentStore,
		SyncStore:              syncStore,
		SearchEngine:           searchEngine,
		ConnectorFactory:       connectorFactory,
		NormaliserReg:          normaliserRegistry,
		Services:               services,
		Logger:                 logger,
		IndexingExecutor:       executor,
		CapabilitySet:          capabilitySet,
		DocumentIngestObserver: observer,
	})

	return orchestrator, sourceStore, documentStore, searchEngine, connectorFactory
}

// TestSyncSource_ObserverHook_NilObserver verifies that when no observer is
// wired, ingest proceeds normally (no panic) and the document is persisted.
func TestSyncSource_ObserverHook_NilObserver(t *testing.T) {
	orchestrator, sourceStore, documentStore, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, nil, nil)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-1",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-1", Title: "Nil Observer Doc"},
					Content:    "content",
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
		t.Fatalf("expected Success=true with nil observer, got error: %s", result.Error)
	}
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}

	// Verify document was persisted via Save.
	saved, err := documentStore.GetByExternalID(ctx, "source-1", "ext-1")
	if err != nil {
		t.Fatalf("expected document to be saved, got error: %v", err)
	}
	if saved.Title != "Nil Observer Doc" {
		t.Errorf("expected title 'Nil Observer Doc', got '%s'", saved.Title)
	}
}

// TestSyncSource_ObserverHook_ReturnsNil verifies that when the observer
// returns nil, ingest succeeds and the observer receives the same (source, doc)
// pair that flowed through the ingest path.
func TestSyncSource_ObserverHook_ReturnsNil(t *testing.T) {
	spy := &spyDocumentIngestObserver{}
	orchestrator, sourceStore, _, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, spy, nil)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-42",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-42", Title: "Observed Doc"},
					Content:    "content",
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
		t.Fatalf("expected Success=true, got error: %s", result.Error)
	}

	// Observer is invoked asynchronously; drain before asserting.
	if err := orchestrator.WaitForObservers(ctx); err != nil {
		t.Fatalf("WaitForObservers: %v", err)
	}

	if got := spy.CallCount(); got != 1 {
		t.Fatalf("expected observer to be called exactly once, got %d", got)
	}

	capturedSource, capturedDoc := spy.Last()
	if capturedSource == nil {
		t.Fatal("expected captured source, got nil")
	}
	if capturedSource.ID != "source-1" {
		t.Errorf("expected captured source ID 'source-1', got '%s'", capturedSource.ID)
	}
	if capturedDoc == nil {
		t.Fatal("expected captured doc, got nil")
	}
	if capturedDoc.ExternalID != "ext-42" {
		t.Errorf("expected captured doc ExternalID 'ext-42', got '%s'", capturedDoc.ExternalID)
	}
	if capturedDoc.Title != "Observed Doc" {
		t.Errorf("expected captured doc Title 'Observed Doc', got '%s'", capturedDoc.Title)
	}
	if capturedDoc.SourceID != "source-1" {
		t.Errorf("expected captured doc SourceID 'source-1' (set in processAddOrUpdate), got '%s'", capturedDoc.SourceID)
	}
}

// TestSyncSource_ObserverHook_ReturnsError_IngestStillSucceeds verifies that
// when the observer returns an error, the ingest still succeeds, the document
// is still saved, and the error is logged via logger.Warn.
func TestSyncSource_ObserverHook_ReturnsError_IngestStillSucceeds(t *testing.T) {
	sentinel := errors.New("observer boom")
	spy := &spyDocumentIngestObserver{returnErr: sentinel}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	orchestrator, sourceStore, documentStore, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, spy, logger)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-err",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-err", Title: "Err Doc"},
					Content:    "content",
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
		t.Fatalf("expected Success=true despite observer error, got error: %s", result.Error)
	}
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("expected 1 document added, got %d", result.Stats.DocumentsAdded)
	}
	if result.Stats.Errors != 0 {
		t.Errorf("expected 0 errors in stats (observer errors are swallowed), got %d", result.Stats.Errors)
	}

	// Document should still be saved (observer runs after Save and doesn't unwind it).
	saved, err := documentStore.GetByExternalID(ctx, "source-1", "ext-err")
	if err != nil {
		t.Fatalf("expected document to be saved despite observer error: %v", err)
	}
	if saved.Title != "Err Doc" {
		t.Errorf("expected title 'Err Doc', got '%s'", saved.Title)
	}

	// Observer is invoked asynchronously; drain before asserting.
	if err := orchestrator.WaitForObservers(ctx); err != nil {
		t.Fatalf("WaitForObservers: %v", err)
	}

	// Observer was invoked.
	if got := spy.CallCount(); got != 1 {
		t.Errorf("expected observer to be called once, got %d", got)
	}

	// Warn log must contain the failure message and the contractual keys.
	logOutput := buf.String()
	if !strings.Contains(logOutput, "document ingest observer failed") {
		t.Errorf("expected log to contain 'document ingest observer failed', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "document_id=") {
		t.Errorf("expected log to contain document_id key, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "source_id=") {
		t.Errorf("expected log to contain source_id key, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "error=") {
		t.Errorf("expected log to contain error key, got: %s", logOutput)
	}
	// The sentinel error's text should surface in the log.
	if !strings.Contains(logOutput, "observer boom") {
		t.Errorf("expected log to contain sentinel error text 'observer boom', got: %s", logOutput)
	}
}

// TestSyncSource_ObserverHook_CalledOncePerSave verifies that ingest of a
// single document invokes the observer exactly once.
func TestSyncSource_ObserverHook_CalledOncePerSave(t *testing.T) {
	spy := &spyDocumentIngestObserver{}
	orchestrator, sourceStore, _, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, spy, nil)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-once",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-once", Title: "Once Doc"},
					Content:    "content",
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
		t.Fatalf("expected Success=true, got error: %s", result.Error)
	}

	// Observer is invoked asynchronously; drain before asserting.
	if err := orchestrator.WaitForObservers(ctx); err != nil {
		t.Fatalf("WaitForObservers: %v", err)
	}

	if got := spy.CallCount(); got != 1 {
		t.Errorf("expected observer call count == 1 for a single successful save, got %d", got)
	}
}

// TestSyncSource_ObserverHook_NotCalledOnSaveFailure verifies that when
// documentStore.Save fails, the observer is not invoked and the ingest
// returns the wrapped save error.
//
// Calls processAddOrUpdate directly (matching TestProcessAddOrUpdate_DocumentFieldsSet)
// because the higher-level SyncSource swallows per-change errors into stats
// and would hide the "ingest returns that error" assertion.
func TestSyncSource_ObserverHook_NotCalledOnSaveFailure(t *testing.T) {
	spy := &spyDocumentIngestObserver{}
	sourceStore := mocks.NewMockSourceStore()
	baseDocStore := mocks.NewMockDocumentStore()
	saveErr := errors.New("document store save failed")
	documentStore := &mockDocumentStoreWithSaveError{
		MockDocumentStore: baseDocStore,
		saveErr:           saveErr,
	}

	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()

	cfg := domain.NewRuntimeConfig("memory")
	services := runtime.NewServices(cfg)

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
		SourceStore:            sourceStore,
		DocumentStore:          documentStore,
		SyncStore:              syncStore,
		SearchEngine:           searchEngine,
		ConnectorFactory:       connectorFactory,
		NormaliserReg:          normaliserRegistry,
		Services:               services,
		IndexingExecutor:       executor,
		CapabilitySet:          capabilitySet,
		DocumentIngestObserver: spy,
	})

	ctx := context.Background()
	source := &domain.Source{ID: "source-1", Enabled: true}
	_ = sourceStore.Save(ctx, source)

	change := &domain.Change{
		Type:       domain.ChangeTypeAdded,
		ExternalID: "ext-fail",
		Document:   &domain.Document{ExternalID: "ext-fail", Title: "Will Fail"},
		Content:    "content",
	}

	stats := &domain.SyncStats{}
	err := orchestrator.processAddOrUpdate(ctx, source, change, stats)
	if err == nil {
		t.Fatal("expected error when documentStore.Save fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to save document") {
		t.Errorf("expected error to wrap save failure ('failed to save document'), got: %v", err)
	}

	if got := spy.CallCount(); got != 0 {
		t.Errorf("expected observer NOT to be called on Save failure, call count = %d", got)
	}

	// Stats should not have incremented (Save failed before the stats block).
	if stats.DocumentsAdded != 0 {
		t.Errorf("expected DocumentsAdded == 0 on Save failure, got %d", stats.DocumentsAdded)
	}
}

// createTestSyncOrchestratorWithDeleteObserver builds a SyncOrchestrator wired
// with the supplied delete observer. Mirrors createTestSyncOrchestratorWithObserver
// but plumbs DocumentDeleteObserver instead so processDelete tests can verify
// the new hook fires.
func createTestSyncOrchestratorWithDeleteObserver(
	t *testing.T,
	observer driven.DocumentDeleteObserver,
) (
	*SyncOrchestrator,
	*mocks.MockSourceStore,
	*mocks.MockDocumentStore,
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
		SourceStore:            sourceStore,
		DocumentStore:          documentStore,
		SyncStore:              syncStore,
		SearchEngine:           searchEngine,
		ConnectorFactory:       connectorFactory,
		NormaliserReg:          normaliserRegistry,
		Services:               services,
		IndexingExecutor:       executor,
		CapabilitySet:          capabilitySet,
		DocumentDeleteObserver: observer,
	})

	return orchestrator, sourceStore, documentStore, searchEngine, connectorFactory
}

// TestSyncSource_DeleteObserverHook_Fires verifies that processDelete invokes
// OnDocumentDeleted on the observer with the source and the captured document
// after the underlying store delete succeeds.
func TestSyncSource_DeleteObserverHook_Fires(t *testing.T) {
	spy := &spyDocumentDeleteObserver{}
	orch, sourceStore, documentStore, _, connectorFactory :=
		createTestSyncOrchestratorWithDeleteObserver(t, spy)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-del",
		Name:         "Del Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)
	_ = documentStore.Save(ctx, &domain.Document{
		ID:         "doc-del",
		SourceID:   "source-del",
		ExternalID: "ext-del",
	})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{ExternalID: "ext-del", Type: domain.ChangeTypeDeleted},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "source-del")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got error %s", result.Error)
	}
	if result.Stats.DocumentsDeleted != 1 {
		t.Fatalf("expected 1 doc deleted, got %d", result.Stats.DocumentsDeleted)
	}

	if spy.docCalls != 1 {
		t.Errorf("OnDocumentDeleted: want 1 call, got %d", spy.docCalls)
	}
	if spy.lastDocSource == nil || spy.lastDocSource.ID != "source-del" {
		t.Errorf("OnDocumentDeleted: source not propagated, got %+v", spy.lastDocSource)
	}
	if spy.lastDoc == nil || spy.lastDoc.ID != "doc-del" {
		t.Errorf("OnDocumentDeleted: doc not propagated, got %+v", spy.lastDoc)
	}
	// Sync path must NOT fire OnSourceDeleted — that's reserved for the
	// admin-cascade path in SourceService.Delete.
	if spy.sourceCalls != 0 {
		t.Errorf("OnSourceDeleted must not fire from sync, got %d calls", spy.sourceCalls)
	}
}

// TestSyncSource_DeleteObserverHook_NilObserver verifies the sync delete path
// runs cleanly when no observer is wired.
func TestSyncSource_DeleteObserverHook_NilObserver(t *testing.T) {
	orch, sourceStore, documentStore, _, connectorFactory :=
		createTestSyncOrchestratorWithDeleteObserver(t, nil)
	ctx := context.Background()

	_ = sourceStore.Save(ctx, &domain.Source{
		ID: "source-nil-del", Name: "Nil Del", ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	})
	_ = documentStore.Save(ctx, &domain.Document{ID: "doc-nil-del", SourceID: "source-nil-del", ExternalID: "ext-x"})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{{ExternalID: "ext-x", Type: domain.ChangeTypeDeleted}}, "", nil
		}
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "source-nil-del")
	if err != nil || !result.Success {
		t.Fatalf("SyncSource with nil delete observer must succeed, got err=%v result=%+v", err, result)
	}
	if result.Stats.DocumentsDeleted != 1 {
		t.Errorf("expected 1 doc deleted, got %d", result.Stats.DocumentsDeleted)
	}
}

// TestSyncSource_DeleteObserverHook_ErrorIgnored exercises the log-and-continue
// posture: an observer error must not propagate up from processDelete.
func TestSyncSource_DeleteObserverHook_ErrorIgnored(t *testing.T) {
	sentinel := errors.New("delete observer boom")
	spy := &spyDocumentDeleteObserver{docReturnErr: sentinel}

	// Capture log output to confirm the warning is emitted.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	orch, sourceStore, documentStore, _, connectorFactory :=
		createTestSyncOrchestratorWithDeleteObserver(t, spy)
	orch.logger = logger
	ctx := context.Background()

	_ = sourceStore.Save(ctx, &domain.Source{
		ID: "src-err-del", Name: "Err Del", ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	})
	_ = documentStore.Save(ctx, &domain.Document{ID: "doc-err-del", SourceID: "src-err-del", ExternalID: "ext-e"})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{{ExternalID: "ext-e", Type: domain.ChangeTypeDeleted}}, "", nil
		}
		return nil, "", nil
	}

	result, err := orch.SyncSource(ctx, "src-err-del")
	if err != nil {
		t.Fatalf("SyncSource must swallow delete-observer errors, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true even with observer error, got %s", result.Error)
	}
	if result.Stats.DocumentsDeleted != 1 {
		t.Errorf("expected 1 doc deleted, got %d", result.Stats.DocumentsDeleted)
	}
	if spy.docCalls != 1 {
		t.Errorf("OnDocumentDeleted: want 1 call, got %d", spy.docCalls)
	}
	if !strings.Contains(buf.String(), "document delete observer failed") {
		t.Errorf("expected warning log on observer error, got: %s", buf.String())
	}
}

// blockingIngestObserver releases on the supplied channel and never returns
// until the test signals it. Used to assert async dispatch — the sync
// returns before the observer completes.
type blockingIngestObserver struct {
	mu       sync.Mutex
	called   int
	release  <-chan struct{}
	observed bool
}

func (b *blockingIngestObserver) OnDocumentIngested(ctx context.Context, _ *domain.Source, _ *domain.Document) error {
	b.mu.Lock()
	b.called++
	b.mu.Unlock()
	select {
	case <-b.release:
		b.mu.Lock()
		b.observed = true
		b.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIngestObserver) Called() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.called
}

func (b *blockingIngestObserver) Observed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.observed
}

// TestSyncSource_AsyncObserver_SyncReturnsBeforeObserverCompletes verifies
// that the sync path returns without waiting for the observer to finish.
// The observer holds its goroutine until released; SyncSource must not
// block on it. WaitForObservers then drains.
func TestSyncSource_AsyncObserver_SyncReturnsBeforeObserverCompletes(t *testing.T) {
	release := make(chan struct{})
	obs := &blockingIngestObserver{release: release}

	orchestrator, sourceStore, _, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, obs, nil)
	ctx := context.Background()

	_ = sourceStore.Save(ctx, &domain.Source{
		ID: "src-async", Name: "Async", ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-async",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-async", Title: "Async Doc"},
					Content:    "content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "src-async")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got %s", result.Error)
	}

	// Observer was dispatched but is still parked on `release`; it must NOT
	// have completed before SyncSource returned.
	if obs.Observed() {
		t.Fatal("observer completed before sync returned — dispatch is not async")
	}

	// Release the observer and drain.
	close(release)
	if err := orchestrator.WaitForObservers(ctx); err != nil {
		t.Fatalf("WaitForObservers: %v", err)
	}

	if got := obs.Called(); got != 1 {
		t.Errorf("expected 1 observer call, got %d", got)
	}
	if !obs.Observed() {
		t.Error("expected observer to record completion after release")
	}
}

// hangingIngestObserver never completes; it just blocks on ctx.Done. Used to
// verify the per-call timeout in dispatchIngestObserver fires.
type hangingIngestObserver struct{}

func (hangingIngestObserver) OnDocumentIngested(ctx context.Context, _ *domain.Source, _ *domain.Document) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestSyncSource_AsyncObserver_TimeoutFiresContextDone verifies that
// observers honouring ctx.Done are cancelled by the per-call timeout.
// We configure a tiny timeout, dispatch a hanging observer, and assert
// WaitForObservers returns once the timeout fires (without external help).
func TestSyncSource_AsyncObserver_TimeoutFiresContextDone(t *testing.T) {
	orchestrator, sourceStore, _, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, hangingIngestObserver{}, nil)
	// Tighten the timeout so the test doesn't wait 30s.
	orchestrator.observerTimeout = 50 * time.Millisecond

	ctx := context.Background()
	_ = sourceStore.Save(ctx, &domain.Source{
		ID: "src-timeout", Name: "Timeout", ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-timeout",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-timeout", Title: "Timeout Doc"},
					Content:    "content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	result, err := orchestrator.SyncSource(ctx, "src-timeout")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got %s", result.Error)
	}

	// WaitForObservers should drain within the per-call timeout window.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer drainCancel()
	start := time.Now()
	if err := orchestrator.WaitForObservers(drainCtx); err != nil {
		t.Fatalf("WaitForObservers: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("WaitForObservers took %v — observer timeout not honoured", elapsed)
	}
}

// TestWaitForObservers_CtxCancel verifies that when a caller passes a
// cancelled context, WaitForObservers returns ctx.Err immediately
// without waiting for observers to finish.
func TestWaitForObservers_CtxCancel(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	obs := &blockingIngestObserver{release: release}
	orchestrator, sourceStore, _, _, connectorFactory :=
		createTestSyncOrchestratorWithObserver(t, obs, nil)
	ctx := context.Background()

	_ = sourceStore.Save(ctx, &domain.Source{
		ID: "src-cancel", Name: "Cancel", ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	})

	connectorFactory.connector.FetchChangesFn = func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "" {
			return []*domain.Change{
				{
					ExternalID: "ext-cancel",
					Type:       domain.ChangeTypeAdded,
					Document:   &domain.Document{ExternalID: "ext-cancel", Title: "Cancel Doc"},
					Content:    "content",
				},
			}, "", nil
		}
		return nil, "", nil
	}

	if _, err := orchestrator.SyncSource(ctx, "src-cancel"); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}

	// Already-cancelled context. WaitForObservers must return immediately
	// with ctx.Err — observer goroutine is still parked on `release`.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := orchestrator.WaitForObservers(cancelCtx); err == nil {
		t.Error("expected context error from WaitForObservers, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
