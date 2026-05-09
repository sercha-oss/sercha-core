package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// ----- fake skip-list store -----

// fakeFailedDocStore is an in-memory SyncFailedDocStore for orchestrator
// tests. Tracks every method call so test bodies can assert wiring,
// recovers a sensible attempt-count progression, and never blocks.
type fakeFailedDocStore struct {
	mu     sync.Mutex
	rows   map[string]*domain.SyncFailedDoc // key: source_id|external_id
	calls  fakeFailedDocStoreCalls
	policy driven.RetryBackoff
	now    time.Time
	// readyOverride lets a test plant rows for the retry pre-pass to
	// pick up without going through Record (handy for the "retry
	// succeeds and clears" path).
	readyOverride []domain.SyncFailedDoc
}

type fakeFailedDocStoreCalls struct {
	record           int
	markSucceeded    int
	listReadyForRetry int
	listBySource     int
	count            int
}

func newFakeFailedDocStore() *fakeFailedDocStore {
	return &fakeFailedDocStore{rows: map[string]*domain.SyncFailedDoc{}}
}

func keyFD(src, ext string) string { return src + "|" + ext }

func (f *fakeFailedDocStore) Record(_ context.Context, failure driven.SyncFailedDocRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.record++
	k := keyFD(failure.SourceID, failure.ExternalID)
	row := f.rows[k]
	if row == nil {
		row = &domain.SyncFailedDoc{
			SourceID:   failure.SourceID,
			ExternalID: failure.ExternalID,
			CreatedAt:  failure.Now,
		}
		f.rows[k] = row
	}
	row.AttemptCount++
	if failure.Err != nil {
		row.LastError = failure.Err.Error()
	}
	row.LastAttemptedAt = failure.Now
	row.NextRetryAfter = failure.Now.Add(failure.Backoff.Base)
	if row.AttemptCount >= failure.Backoff.MaxAttempts {
		row.Terminal = true
	}
	return nil
}

func (f *fakeFailedDocStore) MarkSucceeded(_ context.Context, sourceID, externalID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.markSucceeded++
	delete(f.rows, keyFD(sourceID, externalID))
	return nil
}

func (f *fakeFailedDocStore) ListReadyForRetry(_ context.Context, sourceID string, _ time.Time, limit int) ([]domain.SyncFailedDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.listReadyForRetry++
	if len(f.readyOverride) > 0 {
		out := append([]domain.SyncFailedDoc(nil), f.readyOverride...)
		f.readyOverride = nil
		return out, nil
	}
	out := []domain.SyncFailedDoc{}
	for _, row := range f.rows {
		if row.SourceID != sourceID || row.Terminal {
			continue
		}
		out = append(out, *row)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeFailedDocStore) ListBySource(_ context.Context, sourceID string, _ int) ([]domain.SyncFailedDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.listBySource++
	out := []domain.SyncFailedDoc{}
	for _, row := range f.rows {
		if row.SourceID == sourceID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (f *fakeFailedDocStore) CountBySource(_ context.Context, sourceID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.count++
	n := 0
	for _, row := range f.rows {
		if row.SourceID == sourceID {
			n++
		}
	}
	return n, nil
}

// ----- orchestrator fixture -----

// createOrchestratorWithSkipList wires a SyncOrchestrator with the
// fakeFailedDocStore engaged. Mirrors createTestSyncOrchestrator but
// adds the new dependency so the always-advance behaviour is active.
func createOrchestratorWithSkipList(t *testing.T) (
	*SyncOrchestrator,
	*mocks.MockSourceStore,
	*mocks.MockSyncStateStore,
	*mockConnectorFactory,
	*fakeFailedDocStore,
) {
	t.Helper()

	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	connectorFactory := newMockConnectorFactory()
	normaliserRegistry := mocks.NewMockNormaliserRegistry()
	failedDocStore := newFakeFailedDocStore()

	cfg := domain.NewRuntimeConfig("memory")
	servicesRT := runtime.NewServices(cfg)

	executor := &mockIndexingExecutor{
		executeFn: func(ctx context.Context, pctx *pipeline.IndexingContext, input *pipeline.IndexingInput) (*pipeline.IndexingOutput, error) {
			return &pipeline.IndexingOutput{
				DocumentID: input.DocumentID,
				ChunkIDs:   []string{input.DocumentID + "-chunk-0"},
			}, nil
		},
	}

	orchestrator := NewSyncOrchestrator(SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		Services:         servicesRT,
		IndexingExecutor: executor,
		CapabilitySet:    pipeline.NewCapabilitySet(),
		FailedDocStore:   failedDocStore,
		FailedDocBackoff: driven.RetryBackoff{
			Base:        time.Second,
			Max:         5 * time.Second,
			MaxAttempts: 3,
		},
	})

	return orchestrator, sourceStore, syncStore, connectorFactory, failedDocStore
}

// ----- behaviour tests -----

// One bad doc no longer blocks the cursor. The connector returns two
// changes; the bad one fails (and lands in the skip-list); the good one
// succeeds; the cursor advances past both.
func TestSyncSource_FailedDocStore_AdvancesCursorPastFailure(t *testing.T) {
	orchestrator, sourceStore, syncStore, connectorFactory, failedDocStore := createOrchestratorWithSkipList(t)
	ctx := context.Background()

	source := &domain.Source{
		ID:           "source-1",
		Name:         "src",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(ctx, source)

	good := &domain.Document{ID: "doc-good", ExternalID: "good", Title: "Good", MimeType: "text/plain"}
	bad := &domain.Document{ID: "doc-bad", ExternalID: "bad", Title: "Bad", MimeType: "text/plain"}

	// Two changes on the first batch; nothing on the second so the loop
	// terminates quickly. The "bad" change uses a LoadContent thunk that
	// returns an error — that path takes processAddOrUpdate down its
	// error branch without needing to mock the entire indexing pipeline.
	connectorFactory.connector.FetchChangesFn = func(_ context.Context, _ *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor == "cur-1" {
			return nil, "", nil
		}
		return []*domain.Change{
			{ExternalID: "good", Type: domain.ChangeTypeAdded, Document: good, Content: "good content"},
			{
				ExternalID: "bad", Type: domain.ChangeTypeAdded, Document: bad,
				LoadContent: func(_ context.Context) (string, error) {
					return "", errors.New("upstream rejected the document")
				},
			},
		}, "cur-1", nil
	}

	result, err := orchestrator.SyncSource(ctx, "source-1")
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if !result.Success {
		t.Errorf("Success: got false, want true (cursor should advance even with one bad doc)")
	}
	// Cursor must advance to "cur-1" — the whole point of the change.
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Cursor != "cur-1" {
		t.Errorf("Cursor: got %q, want %q", state.Cursor, "cur-1")
	}
	// Good doc still indexed.
	if result.Stats.DocumentsAdded != 1 {
		t.Errorf("DocumentsAdded: got %d, want 1", result.Stats.DocumentsAdded)
	}
	// Bad doc landed in skip-list.
	if failedDocStore.calls.record != 1 {
		t.Errorf("Record calls: got %d, want 1", failedDocStore.calls.record)
	}
	rows, _ := failedDocStore.ListBySource(ctx, "source-1", 100)
	if len(rows) != 1 || rows[0].ExternalID != "bad" {
		t.Errorf("expected one skip-list row for 'bad'; got %#v", rows)
	}
}

// Successful processing of a previously-failing doc clears its skip-list
// row — the next sync run shouldn't re-retry it.
func TestSyncSource_FailedDocStore_ClearsRowOnSuccess(t *testing.T) {
	orchestrator, sourceStore, _, connectorFactory, failedDocStore := createOrchestratorWithSkipList(t)
	ctx := context.Background()

	source := &domain.Source{
		ID: "source-1", Name: "src",
		ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	// Pre-seed a failing row.
	_ = failedDocStore.Record(ctx, driven.SyncFailedDocRecord{
		SourceID: "source-1", ExternalID: "ext-1",
		Err: errors.New("prior failure"),
		Now: time.Now().UTC(),
		Backoff: driven.RetryBackoff{Base: time.Second, Max: time.Minute, MaxAttempts: 5},
	})
	failedDocStore.calls = fakeFailedDocStoreCalls{} // reset for the assertion below

	doc := &domain.Document{ID: "doc-1", ExternalID: "ext-1", Title: "Now Working", MimeType: "text/plain"}
	// The retry pre-pass fetches the doc fresh via FetchDocument; wire it
	// to return the doc + its content so the synthetic Modified change
	// has everything the pipeline needs.
	connectorFactory.connector.FetchDocumentFn = func(_ context.Context, _ *domain.Source, externalID string) (*domain.Document, string, error) {
		if externalID == "ext-1" {
			return doc, "content", nil
		}
		return nil, "", errors.New("unexpected externalID")
	}
	// FetchChanges has nothing fresh — the only work this run is the
	// retry of the previously-failing doc. Returning empty cursor
	// terminates the loop without processing the same doc twice.
	connectorFactory.connector.FetchChangesFn = func(_ context.Context, _ *domain.Source, _ string) ([]*domain.Change, string, error) {
		return nil, "", nil
	}

	if _, err := orchestrator.SyncSource(ctx, "source-1"); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if failedDocStore.calls.markSucceeded == 0 {
		t.Error("expected MarkSucceeded to be called when a delta processes a previously-failing doc")
	}
	rows, _ := failedDocStore.ListBySource(ctx, "source-1", 100)
	if len(rows) != 0 {
		t.Errorf("skip-list row should be cleared; still have %d", len(rows))
	}
}

// Without a FailedDocStore the legacy "stall cursor on any error"
// behaviour MUST be preserved so existing test wiring and any
// embedder relying on the old semantic doesn't silently change.
func TestSyncSource_LegacyMode_StillStallsCursorOnError(t *testing.T) {
	// createTestSyncOrchestrator wires NO FailedDocStore.
	orchestrator, sourceStore, _, syncStore, _, connectorFactory := createTestSyncOrchestrator(t)
	ctx := context.Background()

	source := &domain.Source{
		ID: "source-1", Name: "src",
		ProviderType: domain.ProviderTypeGitHub, Enabled: true,
	}
	_ = sourceStore.Save(ctx, source)

	doc := &domain.Document{ID: "doc-bad", ExternalID: "bad", Title: "Bad", MimeType: "text/plain"}
	connectorFactory.connector.FetchChangesFn = func(_ context.Context, _ *domain.Source, cursor string) ([]*domain.Change, string, error) {
		if cursor != "" {
			return nil, "", nil
		}
		return []*domain.Change{
			{
				ExternalID: "bad", Type: domain.ChangeTypeAdded, Document: doc,
				LoadContent: func(_ context.Context) (string, error) {
					return "", errors.New("upstream broken")
				},
			},
		}, "cur-1", nil
	}

	if _, err := orchestrator.SyncSource(ctx, "source-1"); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	state, _ := syncStore.Get(ctx, "source-1")
	if state.Cursor == "cur-1" {
		t.Error("legacy mode must NOT advance cursor when a doc fails")
	}
}

// ListFailedDocuments returns nothing when no store is wired (legacy
// mode) — distinct from "no failures" which returns an empty slice.
func TestSyncOrchestrator_ListFailedDocuments_NilWithoutStore(t *testing.T) {
	orchestrator, _, _, _, _, _ := createTestSyncOrchestrator(t)
	got, err := orchestrator.ListFailedDocuments(context.Background(), "source-1", 100)
	if err != nil {
		t.Fatalf("ListFailedDocuments: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil slice in legacy mode, got %v", got)
	}
}

// ListFailedDocuments returns the store's rows when wired.
func TestSyncOrchestrator_ListFailedDocuments_DelegatesToStore(t *testing.T) {
	orchestrator, _, _, _, failedDocStore := createOrchestratorWithSkipList(t)
	ctx := context.Background()
	_ = failedDocStore.Record(ctx, driven.SyncFailedDocRecord{
		SourceID: "source-1", ExternalID: "ext-1",
		Err: errors.New("nope"),
		Now: time.Now().UTC(),
		Backoff: driven.RetryBackoff{Base: time.Second, Max: time.Minute, MaxAttempts: 5},
	})
	got, err := orchestrator.ListFailedDocuments(ctx, "source-1", 100)
	if err != nil {
		t.Fatalf("ListFailedDocuments: %v", err)
	}
	if len(got) != 1 || got[0].ExternalID != "ext-1" {
		t.Errorf("got %#v, want one row for ext-1", got)
	}
}
