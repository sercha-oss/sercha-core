package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// defaultDocConcurrency is the per-container fan-out used when
// SyncOrchestratorConfig.Concurrency is zero. Chosen as a balance between
// embedder/connector RPM headroom and observed wall-time gain on large syncs.
const defaultDocConcurrency = 4

// defaultObserverTimeout bounds each individual OnDocumentIngested
// invocation. A stuck observer cannot indefinitely block the bounded
// goroutine pool — once the timeout fires the dispatched goroutine
// completes and releases its slot. 30s is generous enough for typical
// network observers (identity lookups, audit writes) without parking
// goroutines forever on a hung downstream.
const defaultObserverTimeout = 30 * time.Second

// defaultObserverQueueDepth caps in-flight observer goroutines per
// orchestrator. When the queue is full, the dispatch path blocks
// briefly on the semaphore — this back-pressure protects the runtime
// from unbounded goroutine growth if the observer is slower than
// ingestion. 32 is loose enough that healthy observers never block.
const defaultObserverQueueDepth = 32

// SyncOrchestrator coordinates the document sync pipeline.
// It implements the 7-step sync flow:
//  1. Get source config
//  2. Create connector
//  3. Validate connector
//  4. Get sync state (cursor for incremental sync)
//  5. Fetch documents
//  6. Process each document (normalise → chunk → embed → store → index)
//  7. Update sync cursor
type SyncOrchestrator struct {
	sourceStore            driven.SourceStore
	documentStore          driven.DocumentStore
	syncStore              driven.SyncStateStore
	searchEngine           driven.SearchEngine
	vectorIndex            driven.VectorIndex
	connectorFactory       driven.ConnectorFactory
	normaliserReg          driven.NormaliserRegistry
	services               *runtime.Services
	logger                 *slog.Logger
	indexingExecutor       pipelineport.IndexingExecutor // Required pipeline executor
	capabilitySet          *pipeline.CapabilitySet       // Capabilities for pipeline
	capabilityStore        driven.CapabilityStore        // For fetching capability preferences
	settingsStore          driven.SettingsStore          // For loading team settings
	syncEventRepo          driven.SyncEventRepository    // For audit logging of sync events
	teamID                 string                        // Team ID for settings lookup
	documentIngestObserver driven.DocumentIngestObserver // Optional; nil means no observer.
	documentDeleteObserver driven.DocumentDeleteObserver // Optional; nil means no observer.
	lock                   driven.DistributedLock        // Optional; nil means no per-source serialization (single-instance mode).
	lockTTL                time.Duration                 // TTL for sync locks; ignored by Postgres advisory locks.
	concurrency            int                           // Per-container doc fan-out. 0 means default.
	statsMu                sync.Mutex                    // Guards *domain.SyncStats mutation under doc-level fan-out.
	observerTimeout        time.Duration                 // Per-call timeout for OnDocumentIngested.
	observerSem            chan struct{}                 // Bounded semaphore for in-flight observer goroutines.
	observerWG             sync.WaitGroup                // Tracks in-flight observer goroutines for WaitForObservers.
}

// SyncOrchestratorConfig holds dependencies for SyncOrchestrator.
type SyncOrchestratorConfig struct {
	SourceStore            driven.SourceStore
	DocumentStore          driven.DocumentStore
	SyncStore              driven.SyncStateStore
	SearchEngine           driven.SearchEngine
	VectorIndex            driven.VectorIndex
	ConnectorFactory       driven.ConnectorFactory
	NormaliserReg          driven.NormaliserRegistry
	Services               *runtime.Services
	Logger                 *slog.Logger
	IndexingExecutor       pipelineport.IndexingExecutor // Required pipeline executor
	CapabilitySet          *pipeline.CapabilitySet       // Capabilities for pipeline
	CapabilityStore        driven.CapabilityStore        // For fetching capability preferences
	SettingsStore          driven.SettingsStore          // For loading team settings
	SyncEventRepo          driven.SyncEventRepository    // For audit logging of sync events
	TeamID                 string                        // Team ID for settings lookup
	DocumentIngestObserver driven.DocumentIngestObserver // Optional; nil means no observer.
	DocumentDeleteObserver driven.DocumentDeleteObserver // Optional; nil means no observer.
	Lock                   driven.DistributedLock        // Optional. When set, SyncSource/SyncContainer acquire "sync:source:<id>" before running so concurrent invocations no-op (Skipped=true) instead of racing.
	LockTTL                time.Duration                 // Optional. Defaults to 1h. Ignored by PG advisory locks (which release on connection close).
	Concurrency            int                           // Optional. Per-container doc-level worker count. Defaults to 4 when zero.
	OnDocumentIngestedTimeout time.Duration             // Optional. Per-call timeout for the async DocumentIngestObserver. Defaults to 30s when zero.
	ObserverQueueDepth     int                           // Optional. Bounded goroutine pool depth for the async DocumentIngestObserver. Defaults to 32 when zero.
}

// NewSyncOrchestrator creates a new sync orchestrator.
func NewSyncOrchestrator(cfg SyncOrchestratorConfig) *SyncOrchestrator {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// IndexingExecutor is now required
	if cfg.IndexingExecutor == nil {
		panic("IndexingExecutor is required for SyncOrchestrator")
	}

	lockTTL := cfg.LockTTL
	if lockTTL == 0 {
		lockTTL = time.Hour
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultDocConcurrency
	}

	observerTimeout := cfg.OnDocumentIngestedTimeout
	if observerTimeout <= 0 {
		observerTimeout = defaultObserverTimeout
	}

	queueDepth := cfg.ObserverQueueDepth
	if queueDepth <= 0 {
		queueDepth = defaultObserverQueueDepth
	}

	return &SyncOrchestrator{
		sourceStore:            cfg.SourceStore,
		documentStore:          cfg.DocumentStore,
		syncStore:              cfg.SyncStore,
		searchEngine:           cfg.SearchEngine,
		vectorIndex:            cfg.VectorIndex,
		connectorFactory:       cfg.ConnectorFactory,
		normaliserReg:          cfg.NormaliserReg,
		services:               cfg.Services,
		logger:                 logger,
		indexingExecutor:       cfg.IndexingExecutor,
		capabilitySet:          cfg.CapabilitySet,
		capabilityStore:        cfg.CapabilityStore,
		settingsStore:          cfg.SettingsStore,
		syncEventRepo:          cfg.SyncEventRepo,
		teamID:                 cfg.TeamID,
		documentIngestObserver: cfg.DocumentIngestObserver,
		documentDeleteObserver: cfg.DocumentDeleteObserver,
		lock:                   cfg.Lock,
		lockTTL:                lockTTL,
		concurrency:            concurrency,
		observerTimeout:        observerTimeout,
		observerSem:            make(chan struct{}, queueDepth),
	}
}

// WaitForObservers blocks until every dispatched DocumentIngestObserver
// goroutine has returned, or until ctx is cancelled. Useful for tests
// that need to assert on observer side-effects after a sync completes,
// and for graceful shutdown paths that want to drain in-flight callbacks
// before tearing down dependencies (database connections, etc.).
//
// Returns nil on clean drain, ctx.Err() on cancel/timeout. Observer
// errors are still logged-and-swallowed by the dispatch goroutine; this
// method only reports caller-side cancellation.
func (o *SyncOrchestrator) WaitForObservers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		o.observerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// withStats serializes mutations to *domain.SyncStats so the doc-level
// worker pool in syncContainer can call processChange concurrently. The
// caller's fn runs under statsMu; keep it short and free of I/O.
func (o *SyncOrchestrator) withStats(stats *domain.SyncStats, fn func(*domain.SyncStats)) {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	fn(stats)
}

// readStatsErrors returns stats.Errors under the stats mutex. Used to
// snapshot the pre/post error count around the doc-level fan-out so the
// cursor advance gate sees a consistent value.
func (o *SyncOrchestrator) readStatsErrors(stats *domain.SyncStats) int {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	return stats.Errors
}

// dispatchIngestObserver fires the configured DocumentIngestObserver in
// a bounded goroutine pool. The dispatch path:
//
//   - acquires observerSem (back-pressure if the queue is full),
//   - increments observerWG so WaitForObservers can drain,
//   - derives a fresh context (detached from the sync request) bounded
//     by observerTimeout, so a stuck observer cannot park its goroutine
//     indefinitely,
//   - logs phase=ingest_observer with duration on completion or failure.
//
// observer.OnDocumentIngested is now invoked from N concurrent goroutines
// (one per in-flight document), so observer implementations must be
// goroutine-safe — see the port godoc.
func (o *SyncOrchestrator) dispatchIngestObserver(source *domain.Source, doc *domain.Document) {
	o.observerSem <- struct{}{}
	o.observerWG.Add(1)
	go func() {
		defer o.observerWG.Done()
		defer func() { <-o.observerSem }()

		ctx, cancel := context.WithTimeout(context.Background(), o.observerTimeout)
		defer cancel()

		obsStart := time.Now()
		err := o.documentIngestObserver.OnDocumentIngested(ctx, source, doc)
		obsDuration := time.Since(obsStart)
		if err != nil {
			o.logger.Warn("document ingest observer failed",
				"phase", "ingest_observer",
				"document_id", doc.ID,
				"source_id", source.ID,
				"duration_ms", obsDuration.Milliseconds(),
				"error", err,
			)
			return
		}
		o.logger.Debug("document ingest observer completed",
			"phase", "ingest_observer",
			"document_id", doc.ID,
			"source_id", source.ID,
			"duration_ms", obsDuration.Milliseconds(),
		)
	}()
}

// acquireSourceLock takes the per-source advisory lock if a lock backend is
// configured. Returns (acquired, release). If no lock is configured, returns
// (true, no-op release) — single-instance mode.
//
// The lock is keyed by source ID, so a sync_source task and a sync_container
// task targeting the same source mutually exclude. This prevents the race
// observed in production where a worker picked up sync_container at T+0 and
// another picked up sync_source at T+2s, double-cleaning chunks and racing
// document writes.
func (o *SyncOrchestrator) acquireSourceLock(ctx context.Context, sourceID string) (bool, func()) {
	if o.lock == nil {
		return true, func() {}
	}
	name := "sync:source:" + sourceID
	acquired, err := o.lock.Acquire(ctx, name, o.lockTTL)
	if err != nil {
		o.logger.Warn("failed to acquire sync lock; proceeding without serialization",
			"source_id", sourceID, "error", err)
		return true, func() {}
	}
	if !acquired {
		return false, func() {}
	}
	return true, func() {
		if err := o.lock.Release(ctx, name); err != nil {
			o.logger.Warn("failed to release sync lock", "source_id", sourceID, "error", err)
		}
	}
}

// SyncContainer synchronizes a single container within a source.
// This is used for incremental updates when containers are added.
func (o *SyncOrchestrator) SyncContainer(ctx context.Context, sourceID, containerID string) (*domain.SyncResult, error) {
	startTime := time.Now()

	acquired, release := o.acquireSourceLock(ctx, sourceID)
	if !acquired {
		o.logger.Info("sync already in progress for source; skipping container sync",
			"source_id", sourceID, "container_id", containerID)
		return &domain.SyncResult{
			SourceID: sourceID,
			Success:  true,
			Skipped:  true,
			Duration: time.Since(startTime).Seconds(),
		}, nil
	}
	defer release()

	o.logger.Info("starting container sync", "source_id", sourceID, "container_id", containerID)

	// Check if sync is enabled in settings
	settings, err := o.loadSettings(ctx)
	if err == nil && !settings.SyncEnabled {
		o.logger.Info("sync disabled in settings", "source_id", sourceID, "container_id", containerID)
		return &domain.SyncResult{
			SourceID: sourceID,
			Success:  false,
			Error:    "sync is disabled in team settings",
			Duration: time.Since(startTime).Seconds(),
		}, nil
	}

	// Step 1: Get source config
	source, err := o.sourceStore.Get(ctx, sourceID)
	if err != nil {
		return o.failSync(ctx, sourceID, startTime, fmt.Errorf("failed to get source: %w", err))
	}

	if !source.Enabled {
		return o.failSync(ctx, sourceID, startTime, fmt.Errorf("source is disabled"))
	}

	// Step 2: Get sync state
	syncState, err := o.syncStore.Get(ctx, sourceID)
	if err != nil {
		// Create initial sync state
		syncState = &domain.SyncState{
			SourceID: sourceID,
			Status:   domain.SyncStatusIdle,
			Stats:    domain.SyncStats{},
		}
	}

	// Mark as running
	now := time.Now()
	syncState.Status = domain.SyncStatusRunning
	syncState.StartedAt = &now
	syncState.Error = ""
	if err := o.syncStore.Save(ctx, syncState); err != nil {
		o.logger.Warn("failed to update sync state to running", "error", err)
	}

	// Track processed external IDs for this container
	processedExternalIDs := make(map[string]bool)

	// Step 3: Sync the specific container
	stats, cursor, err := o.syncContainer(ctx, source, syncState, containerID, processedExternalIDs)
	if err != nil {
		o.logger.Error("container sync failed",
			"source_id", sourceID,
			"container_id", containerID,
			"error", err,
		)
		return o.failSync(ctx, sourceID, startTime, fmt.Errorf("container sync failed: %w", err))
	}

	// Step 4: Update final sync state
	completedAt := time.Now()
	syncState.Status = domain.SyncStatusCompleted
	syncState.Error = ""
	syncState.LastSyncAt = &completedAt
	syncState.CompletedAt = &completedAt
	syncState.Cursor = cursor
	// Don't overwrite stats - this is just one container
	// In a production system, you might want container-specific sync states

	if err := o.syncStore.Save(ctx, syncState); err != nil {
		o.logger.Warn("failed to update sync state", "error", err)
	}

	duration := time.Since(startTime).Seconds()

	o.logger.Info("container sync completed",
		"source_id", sourceID,
		"container_id", containerID,
		"duration_seconds", duration,
		"documents_added", stats.DocumentsAdded,
		"documents_updated", stats.DocumentsUpdated,
		"documents_deleted", stats.DocumentsDeleted,
		"chunks_indexed", stats.ChunksIndexed,
		"errors", stats.Errors,
	)

	return &domain.SyncResult{
		SourceID: sourceID,
		Success:  true,
		Stats:    *stats,
		Duration: duration,
		Cursor:   cursor,
	}, nil
}

// SyncSource synchronizes a single source.
// This is the main entry point for the sync pipeline.
// For sources with container selection, it syncs each selected container.
func (o *SyncOrchestrator) SyncSource(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
	startTime := time.Now()

	acquired, release := o.acquireSourceLock(ctx, sourceID)
	if !acquired {
		o.logger.Info("sync already in progress for source; skipping", "source_id", sourceID)
		return &domain.SyncResult{
			SourceID: sourceID,
			Success:  true,
			Skipped:  true,
			Duration: time.Since(startTime).Seconds(),
		}, nil
	}
	defer release()

	o.logger.Info("starting sync", "source_id", sourceID)

	// Check if sync is enabled in settings
	settings, err := o.loadSettings(ctx)
	if err == nil && !settings.SyncEnabled {
		o.logger.Info("sync disabled in settings", "source_id", sourceID)
		return &domain.SyncResult{
			SourceID: sourceID,
			Success:  false,
			Error:    "sync is disabled in team settings",
			Duration: time.Since(startTime).Seconds(),
		}, nil
	}

	// Step 1: Get source config
	source, err := o.sourceStore.Get(ctx, sourceID)
	if err != nil {
		return o.failSync(ctx, sourceID, startTime, fmt.Errorf("failed to get source: %w", err))
	}

	if !source.Enabled {
		return o.failSync(ctx, sourceID, startTime, fmt.Errorf("source is disabled"))
	}

	// Step 2: Get sync state
	syncState, err := o.syncStore.Get(ctx, sourceID)
	if err != nil {
		// Create initial sync state
		syncState = &domain.SyncState{
			SourceID: sourceID,
			Status:   domain.SyncStatusIdle,
			Stats:    domain.SyncStats{},
		}
	}

	// Mark as running
	now := time.Now()
	syncState.Status = domain.SyncStatusRunning
	syncState.StartedAt = &now
	syncState.Error = ""
	if err := o.syncStore.Save(ctx, syncState); err != nil {
		o.logger.Warn("failed to update sync state to running", "error", err)
	}

	// Determine containers to sync
	// If selected containers are specified, sync each one
	// Otherwise, sync with empty containerID (provider indexes all content)
	var containerIDs []string
	if len(source.Containers) > 0 {
		containerIDs = make([]string, len(source.Containers))
		for i, c := range source.Containers {
			containerIDs[i] = c.ID
		}
	} else {
		containerIDs = []string{""} // Empty string means sync all accessible content
	}

	// Aggregate stats across all containers
	aggregatedStats := domain.SyncStats{}
	var lastCursor string
	var syncErrors []string

	// Track processed external IDs across containers to avoid duplicate processing
	// This is needed when the same document appears in multiple containers
	// (e.g., a page that's both a specific container AND an entry in a database container)
	processedExternalIDs := make(map[string]bool)

	// Full sync (no cursor): wipe all existing indexed data for this source ONCE
	// before processing any containers. This prevents orphaned chunks from
	// accumulating across re-syncs.
	if syncState.Cursor == "" {
		o.logger.Info("full sync: clearing existing indexed data", "source_id", source.ID)
		if o.searchEngine != nil {
			if err := o.searchEngine.DeleteBySource(ctx, source.ID); err != nil {
				o.logger.Warn("failed to clear search engine data for source", "source_id", source.ID, "error", err)
			}
		}
		if o.vectorIndex != nil {
			// Delete embeddings for all documents in this source
			docs, err := o.documentStore.GetBySource(ctx, source.ID, 10000, 0)
			if err == nil {
				docIDs := make([]string, len(docs))
				for i, d := range docs {
					docIDs[i] = d.ID
				}
				if len(docIDs) > 0 {
					_ = o.vectorIndex.DeleteByDocuments(ctx, docIDs)
				}
			}
		}
	}

	// Step 3: Sync each container
	for _, containerID := range containerIDs {
		containerStats, cursor, err := o.syncContainer(ctx, source, syncState, containerID, processedExternalIDs)
		if err != nil {
			o.logger.Error("container sync failed",
				"source_id", sourceID,
				"container_id", containerID,
				"error", err,
			)
			syncErrors = append(syncErrors, fmt.Sprintf("%s: %s", containerID, err.Error()))
			aggregatedStats.Errors++
			continue
		}

		// Aggregate stats
		aggregatedStats.DocumentsAdded += containerStats.DocumentsAdded
		aggregatedStats.DocumentsUpdated += containerStats.DocumentsUpdated
		aggregatedStats.DocumentsDeleted += containerStats.DocumentsDeleted
		aggregatedStats.ChunksIndexed += containerStats.ChunksIndexed
		aggregatedStats.Errors += containerStats.Errors

		if cursor != "" {
			lastCursor = cursor // Use last non-empty cursor
		}
	}

	// Step 4: Update final sync state
	completedAt := time.Now()
	if len(syncErrors) > 0 && len(syncErrors) == len(containerIDs) {
		// All containers failed
		syncState.Status = domain.SyncStatusFailed
		syncState.Error = fmt.Sprintf("all containers failed: %v", syncErrors)
	} else if len(syncErrors) > 0 {
		// Partial failure
		syncState.Status = domain.SyncStatusCompleted // Still mark as completed
		syncState.Error = fmt.Sprintf("partial failure: %v", syncErrors)
	} else {
		syncState.Status = domain.SyncStatusCompleted
		syncState.Error = ""
	}

	syncState.LastSyncAt = &completedAt
	syncState.CompletedAt = &completedAt
	syncState.Cursor = lastCursor
	syncState.Stats = aggregatedStats

	if err := o.syncStore.Save(ctx, syncState); err != nil {
		o.logger.Warn("failed to update sync state", "error", err)
	}

	duration := time.Since(startTime).Seconds()

	o.logger.Info("sync completed",
		"source_id", sourceID,
		"containers_count", len(containerIDs),
		"duration_seconds", duration,
		"documents_added", aggregatedStats.DocumentsAdded,
		"documents_updated", aggregatedStats.DocumentsUpdated,
		"documents_deleted", aggregatedStats.DocumentsDeleted,
		"chunks_indexed", aggregatedStats.ChunksIndexed,
		"errors", aggregatedStats.Errors,
	)

	// Log sync event for audit trail
	if o.syncEventRepo != nil {
		syncEvent := domain.NewSyncEvent(
			o.teamID,
			sourceID,
			source.Name,
			source.ProviderType,
			syncState.Status,
			aggregatedStats,
			duration,
		)
		if syncState.Error != "" {
			syncEvent.WithError(syncState.Error)
		}
		if err := o.syncEventRepo.Save(ctx, syncEvent); err != nil {
			o.logger.Warn("failed to save sync event", "error", err)
		}
	}

	success := syncState.Status == domain.SyncStatusCompleted && syncState.Error == ""
	return &domain.SyncResult{
		SourceID: sourceID,
		Success:  success,
		Stats:    aggregatedStats,
		Duration: duration,
		Cursor:   lastCursor,
		Error:    syncState.Error,
	}, nil
}

// syncContainer syncs a single container within a source.
// Returns stats for this container, the cursor, and any error.
// processedExternalIDs tracks documents already processed by previous containers
// to avoid duplicate processing within the same sync operation.
func (o *SyncOrchestrator) syncContainer(
	ctx context.Context,
	source *domain.Source,
	syncState *domain.SyncState,
	containerID string,
	processedExternalIDs map[string]bool,
) (*domain.SyncStats, string, error) {
	logFields := []any{"source_id", source.ID}
	if containerID != "" {
		logFields = append(logFields, "container_id", containerID)
	}
	o.logger.Info("syncing container", logFields...)

	// Create connector scoped to this container
	connector, err := o.connectorFactory.Create(ctx, source, containerID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create connector: %w", err)
	}

	// Test connection
	if err := connector.TestConnection(ctx, source); err != nil {
		return nil, "", fmt.Errorf("connection test failed: %w", err)
	}

	// Use container-specific cursor if available
	cursor := syncState.Cursor
	stats := &domain.SyncStats{}
	var lastCursor string

	// Note: Full sync clearing is now handled once in Sync() before the container loop,
	// not per-container here. This prevents wiping previously-indexed container data.

	// Phase 1: reconcile deletes for snapshot-served scopes.
	//
	// For each scope the connector declares, compare the connector's current
	// Inventory against everything we have indexed under that prefix. Orphans
	// (indexed but not present upstream) get emitted as ChangeTypeDeleted and
	// flow through the same processChange path as any other delete. The cursor
	// is NOT advanced here — phase 1 is cleanup, and any failure must not
	// block phase 2 or corrupt the delta watermark.
	//
	// Connectors with native delete signals (e.g. OneDrive) return no scopes;
	// the loop does nothing for them.
	o.reconcileDeletions(ctx, source, connector, stats, processedExternalIDs)

	for {
		select {
		case <-ctx.Done():
			return stats, lastCursor, ctx.Err()
		default:
		}

		fetchStart := time.Now()
		changes, nextCursor, err := connector.FetchChanges(ctx, source, cursor)
		fetchDuration := time.Since(fetchStart)
		if err != nil {
			o.logger.Warn("connector fetch_changes failed",
				"phase", "fetch_changes",
				"source_id", source.ID,
				"container_id", containerID,
				"duration_ms", fetchDuration.Milliseconds(),
				"error", err,
			)
			return stats, lastCursor, fmt.Errorf("failed to fetch changes: %w", err)
		}
		o.logger.Info("connector fetch_changes completed",
			"phase", "fetch_changes",
			"source_id", source.ID,
			"container_id", containerID,
			"changes", len(changes),
			"duration_ms", fetchDuration.Milliseconds(),
		)

		if len(changes) == 0 {
			break
		}

		// Filter out already-processed documents (from other containers in this sync)
		var filteredChanges []*domain.Change
		for _, change := range changes {
			if processedExternalIDs[change.ExternalID] {
				o.logger.Debug("skipping already-processed document",
					"source_id", source.ID,
					"container_id", containerID,
					"external_id", change.ExternalID,
				)
				continue
			}
			filteredChanges = append(filteredChanges, change)
		}
		changes = filteredChanges

		if len(changes) == 0 {
			// All changes were duplicates, continue to next batch
			if nextCursor == "" || nextCursor == cursor {
				break
			}
			cursor = nextCursor
			continue
		}

		// Collect document IDs that need old-chunk cleanup (updates/modifications)
		var updateDocIDs []string
		for _, change := range changes {
			if change.Type == domain.ChangeTypeModified {
				existingDoc, _ := o.documentStore.GetByExternalID(ctx, source.ID, change.ExternalID)
				if existingDoc != nil {
					updateDocIDs = append(updateDocIDs, existingDoc.ID)
				}
			}
		}

		// Bulk delete old chunks/embeddings for all updates in one shot
		o.cleanupOldChunksBatch(ctx, updateDocIDs)

		// Process each document with bounded fan-out. Per-doc errors are
		// counted via stats.Errors and must NOT short-circuit the errgroup —
		// one bad doc cannot abort siblings. Cursor advance is gated by the
		// post-batch error count comparison below.
		//
		// processedExternalIDs is updated synchronously before fan-out so it
		// stays a single-writer map.
		for _, change := range changes {
			processedExternalIDs[change.ExternalID] = true
		}

		errorsBefore := o.readStatsErrors(stats)

		g, gctx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, o.concurrency)
		for _, change := range changes {
			change := change
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()

				procStart := time.Now()
				err := o.processChange(gctx, source, change, stats)
				procDuration := time.Since(procStart)
				if err != nil {
					o.logger.Warn("failed to process change",
						"phase", "process_change",
						"source_id", source.ID,
						"container_id", containerID,
						"external_id", change.ExternalID,
						"duration_ms", procDuration.Milliseconds(),
						"error", err,
					)
					o.withStats(stats, func(s *domain.SyncStats) { s.Errors++ })
				} else {
					o.logger.Debug("processed change",
						"phase", "process_change",
						"source_id", source.ID,
						"container_id", containerID,
						"external_id", change.ExternalID,
						"change_type", string(change.Type),
						"duration_ms", procDuration.Milliseconds(),
					)
				}
				return nil // never short-circuit on per-doc error
			})
		}
		// Wait returns the first non-nil error from g.Go. Since the goroutines
		// always return nil, this is effectively a barrier; it propagates only
		// the gctx cancellation if the parent ctx is cancelled.
		if err := g.Wait(); err != nil {
			return stats, lastCursor, err
		}

		// Only advance cursor if all documents in this batch succeeded
		errorsAfter := o.readStatsErrors(stats)
		if errorsAfter == errorsBefore {
			lastCursor = nextCursor
		} else {
			o.logger.Warn("not advancing cursor due to failed documents",
				"source_id", source.ID,
				"container_id", containerID,
				"failed_count", errorsAfter-errorsBefore,
			)
		}

		// No more pages
		if nextCursor == "" || nextCursor == cursor {
			break
		}
		cursor = nextCursor
	}

	o.logger.Info("container sync completed",
		"source_id", source.ID,
		"container_id", containerID,
		"documents_added", stats.DocumentsAdded,
		"documents_updated", stats.DocumentsUpdated,
		"documents_deleted", stats.DocumentsDeleted,
	)

	return stats, lastCursor, nil
}

// SyncAll synchronizes all enabled sources for a team.
func (o *SyncOrchestrator) SyncAll(ctx context.Context) ([]*domain.SyncResult, error) {
	sources, err := o.sourceStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	var results []*domain.SyncResult
	for _, source := range sources {
		if !source.Enabled {
			continue
		}

		result, err := o.SyncSource(ctx, source.ID)
		if err != nil {
			o.logger.Error("sync failed", "source_id", source.ID, "error", err)
			results = append(results, &domain.SyncResult{
				SourceID: source.ID,
				Success:  false,
				Error:    err.Error(),
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// processChange processes a single document change.
func (o *SyncOrchestrator) processChange(
	ctx context.Context,
	source *domain.Source,
	change *domain.Change,
	stats *domain.SyncStats,
) error {
	switch change.Type {
	case domain.ChangeTypeDeleted:
		return o.processDelete(ctx, source, change, stats)
	case domain.ChangeTypeAdded, domain.ChangeTypeModified:
		return o.processAddOrUpdate(ctx, source, change, stats)
	default:
		return fmt.Errorf("unknown change type: %s", change.Type)
	}
}

// processDelete handles document deletion across all storage layers.
func (o *SyncOrchestrator) processDelete(
	ctx context.Context,
	source *domain.Source,
	change *domain.Change,
	stats *domain.SyncStats,
) error {
	doc, err := o.documentStore.GetByExternalID(ctx, source.ID, change.ExternalID)
	if err != nil {
		return nil
	}

	// Delete embeddings from vector index
	if o.vectorIndex != nil {
		if err := o.vectorIndex.DeleteByDocument(ctx, doc.ID); err != nil {
			o.logger.Warn("failed to delete embeddings", "doc_id", doc.ID, "error", err)
		}
	}

	// Delete from search engine (OpenSearch)
	if o.searchEngine != nil {
		if err := o.searchEngine.DeleteByDocument(ctx, doc.ID); err != nil {
			o.logger.Warn("failed to delete from search engine", "doc_id", doc.ID, "error", err)
		}
	}

	// Delete document (source of truth)
	if err := o.documentStore.Delete(ctx, doc.ID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// Observer fires only after successful deletion. Failures are logged and
	// ignored — observer health must not affect deletion correctness, mirroring
	// the ingest observer posture.
	if o.documentDeleteObserver != nil {
		if err := o.documentDeleteObserver.OnDocumentDeleted(ctx, source, doc); err != nil {
			o.logger.Warn("document delete observer failed",
				"document_id", doc.ID,
				"source_id", source.ID,
				"error", err,
			)
		}
	}

	o.withStats(stats, func(s *domain.SyncStats) { s.DocumentsDeleted++ })
	return nil
}

// reconcileDeletions compares the connector's current Inventory against the
// document store for each declared ReconciliationScope. External IDs present
// in the store but absent from the inventory are treated as deletions
// upstream and routed through processDelete.
//
// Failures are logged and tolerated — a phase-1 failure must not block
// phase-2 add/update work, and the next tick will reconcile whatever got
// missed. The cursor is not touched.
func (o *SyncOrchestrator) reconcileDeletions(
	ctx context.Context,
	source *domain.Source,
	connector driven.Connector,
	stats *domain.SyncStats,
	processedExternalIDs map[string]bool,
) {
	scopes := connector.ReconciliationScopes()
	if len(scopes) == 0 {
		return
	}

	storedIDs, err := o.documentStore.ListExternalIDs(ctx, source.ID)
	if err != nil {
		o.logger.Warn("reconcile: failed to list stored external IDs; skipping",
			"source_id", source.ID, "error", err)
		return
	}

	for _, scope := range scopes {
		var storedInScope []string
		for _, id := range storedIDs {
			if strings.HasPrefix(id, scope) {
				storedInScope = append(storedInScope, id)
			}
		}
		if len(storedInScope) == 0 {
			continue
		}

		inventory, err := connector.Inventory(ctx, source, scope)
		if err != nil {
			o.logger.Warn("reconcile: inventory failed; skipping scope",
				"source_id", source.ID, "scope", scope, "error", err)
			continue
		}

		inventorySet := make(map[string]struct{}, len(inventory))
		for _, id := range inventory {
			inventorySet[id] = struct{}{}
		}

		// Emit deletes for any stored ID not present in the current inventory.
		// A wholesale wipe (zero inventory vs non-zero stored) is possible but
		// suspicious — log it loudly so operators can spot a misconfigured
		// connector or permission loss before it chews through the index.
		if len(inventory) == 0 {
			o.logger.Warn("reconcile: inventory is empty but store is not — verify connector access",
				"source_id", source.ID, "scope", scope, "stored_count", len(storedInScope))
		}

		for _, extID := range storedInScope {
			if _, present := inventorySet[extID]; present {
				continue
			}
			if processedExternalIDs[extID] {
				continue
			}
			processedExternalIDs[extID] = true

			change := &domain.Change{
				Type:       domain.ChangeTypeDeleted,
				ExternalID: extID,
			}
			if err := o.processChange(ctx, source, change, stats); err != nil {
				o.logger.Warn("reconcile: processDelete failed",
					"source_id", source.ID, "scope", scope, "external_id", extID, "error", err)
				o.withStats(stats, func(s *domain.SyncStats) { s.Errors++ })
			}
		}
	}
}

// cleanupOldChunksBatch removes old search index entries and embeddings for multiple
// documents in a single bulk operation. This is much faster than per-document deletes.
// Note: Chunks live in OpenSearch (not PostgreSQL), embeddings live in pgvector.
func (o *SyncOrchestrator) cleanupOldChunksBatch(ctx context.Context, documentIDs []string) {
	if len(documentIDs) == 0 {
		return
	}

	o.logger.Info("cleaning up old chunks before re-indexing", "document_count", len(documentIDs))

	// Bulk delete from search engine (OpenSearch)
	if o.searchEngine != nil {
		if err := o.searchEngine.DeleteByDocuments(ctx, documentIDs); err != nil {
			o.logger.Warn("failed to bulk delete old chunks from search engine", "error", err)
		}
	}

	// Bulk delete embeddings from vector index (pgvector)
	if o.vectorIndex != nil {
		if err := o.vectorIndex.DeleteByDocuments(ctx, documentIDs); err != nil {
			o.logger.Warn("failed to bulk delete old embeddings", "error", err)
		}
	}
}

// processAddOrUpdate handles document creation or update.
func (o *SyncOrchestrator) processAddOrUpdate(
	ctx context.Context,
	source *domain.Source,
	change *domain.Change,
	stats *domain.SyncStats,
) error {
	doc := change.Document
	content := change.Content

	// Lazy content resolution. When a connector populates LoadContent,
	// the eager Content field is empty and the actual download happens
	// here — concurrently across documents under the per-container worker
	// pool. The pre-thunk listing in FetchChanges stays metadata-only and
	// returns quickly, eliminating the head-of-line wait that previously
	// delayed every doc behind the slowest connector listing.
	if change.LoadContent != nil {
		loadStart := time.Now()
		loaded, err := change.LoadContent(ctx)
		loadDuration := time.Since(loadStart)
		if err != nil {
			o.logger.Warn("connector load_content failed",
				"phase", "load_content",
				"source_id", source.ID,
				"external_id", change.ExternalID,
				"duration_ms", loadDuration.Milliseconds(),
				"error", err,
			)
			return fmt.Errorf("load content: %w", err)
		}
		o.logger.Debug("connector load_content completed",
			"phase", "load_content",
			"source_id", source.ID,
			"external_id", change.ExternalID,
			"bytes", len(loaded),
			"duration_ms", loadDuration.Milliseconds(),
		)
		content = loaded
	}

	if doc == nil {
		return fmt.Errorf("document is nil for change type %s", change.Type)
	}

	// Check if document exists (for update tracking)
	existingDoc, _ := o.documentStore.GetByExternalID(ctx, source.ID, change.ExternalID)
	isUpdate := existingDoc != nil

	// Ensure document has required fields
	if doc.ID == "" {
		doc.ID = generateID()
	}
	doc.SourceID = source.ID
	doc.ExternalID = change.ExternalID
	now := time.Now()
	doc.UpdatedAt = now
	doc.IndexedAt = now
	if !isUpdate {
		doc.CreatedAt = now
	} else {
		doc.ID = existingDoc.ID // Preserve original ID
		doc.CreatedAt = existingDoc.CreatedAt
	}

	// Step 6a: Check exclusion rules
	if o.shouldExclude(ctx, doc) {
		o.logger.Debug("document excluded by sync exclusion pattern",
			"source_id", source.ID,
			"path", doc.Path,
		)
		// Don't count as error, just skip
		return nil
	}

	// Step 6b: Normalise content
	normalizedContent := content
	normaliser := o.normaliserReg.Get(doc.MimeType)
	if normaliser != nil {
		normalizedContent = normaliser.Normalise(normalizedContent, doc.MimeType)
	}

	// Process with pipeline executor (required)
	return o.processWithPipeline(ctx, source, doc, normalizedContent, isUpdate, stats)
}

// processWithPipeline processes a document using the pipeline executor.
func (o *SyncOrchestrator) processWithPipeline(
	ctx context.Context,
	source *domain.Source,
	doc *domain.Document,
	content string,
	isUpdate bool,
	stats *domain.SyncStats,
) error {
	// Convert metadata from map[string]string to map[string]any
	metadata := make(map[string]any)
	for k, v := range doc.Metadata {
		metadata[k] = v
	}

	// Build pipeline input
	pipelineInput := &pipeline.IndexingInput{
		DocumentID: doc.ID,
		SourceID:   source.ID,
		Title:      doc.Title,
		Content:    content,
		MimeType:   doc.MimeType,
		Path:       doc.Path,
		Metadata:   metadata,
	}

	// Build pipeline context
	pipelineContext := &pipeline.IndexingContext{
		PipelineID:   "default-indexing",
		ConnectorID:  source.ID,
		SourceID:     source.ID,
		Capabilities: o.capabilitySet,
		Metadata:     make(map[string]any),
	}

	// Fetch capability preferences
	if o.capabilityStore != nil {
		// Use "default" teamID - in production, this should come from source metadata
		prefs, _ := o.capabilityStore.GetPreferences(ctx, "default")
		if prefs != nil {
			pipelineContext.Preferences = &pipeline.StagePreferences{
				TextIndexingEnabled:      prefs.TextIndexingEnabled,
				EmbeddingIndexingEnabled: prefs.EmbeddingIndexingEnabled,
				BM25SearchEnabled:        prefs.BM25SearchEnabled,
				VectorSearchEnabled:      prefs.VectorSearchEnabled,
			}
		}
	}

	// Execute pipeline
	output, err := o.indexingExecutor.Execute(ctx, pipelineContext, pipelineInput)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Save document metadata (pipeline already stored chunks)
	if err := o.documentStore.Save(ctx, doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Observer fires only after successful persistence. The call dispatches
	// onto a bounded goroutine pool and returns immediately so the per-doc
	// processing path doesn't block on observer latency. Failures are logged
	// and ignored — observer health must not affect ingest correctness.
	if o.documentIngestObserver != nil {
		o.dispatchIngestObserver(source, doc)
	}

	// Update stats
	chunkCount := len(output.ChunkIDs)
	o.withStats(stats, func(s *domain.SyncStats) {
		if isUpdate {
			s.DocumentsUpdated++
		} else {
			s.DocumentsAdded++
		}
		s.ChunksIndexed += chunkCount
	})

	return nil
}

// failSync marks a sync as failed and returns the result.
func (o *SyncOrchestrator) failSync(
	ctx context.Context,
	sourceID string,
	startTime time.Time,
	err error,
) (*domain.SyncResult, error) {
	duration := time.Since(startTime).Seconds()

	o.logger.Error("sync failed", "source_id", sourceID, "duration_seconds", duration, "error", err)

	// Update sync state
	syncState, getErr := o.syncStore.Get(ctx, sourceID)
	if getErr == nil {
		now := time.Now()
		syncState.Status = domain.SyncStatusFailed
		syncState.CompletedAt = &now
		syncState.Error = err.Error()
		_ = o.syncStore.Save(ctx, syncState)
	}

	// Log sync event for audit trail
	if o.syncEventRepo != nil {
		// Get source info for event logging
		source, sourceErr := o.sourceStore.Get(ctx, sourceID)
		if sourceErr == nil {
			syncEvent := domain.NewSyncEvent(
				o.teamID,
				sourceID,
				source.Name,
				source.ProviderType,
				domain.SyncStatusFailed,
				domain.SyncStats{}, // Empty stats for failed sync
				duration,
			).WithError(err.Error())
			if saveErr := o.syncEventRepo.Save(ctx, syncEvent); saveErr != nil {
				o.logger.Warn("failed to save sync event", "error", saveErr)
			}
		}
	}

	return &domain.SyncResult{
		SourceID: sourceID,
		Success:  false,
		Error:    err.Error(),
		Duration: duration,
	}, err
}

// GetSyncState retrieves the sync state for a source.
func (o *SyncOrchestrator) GetSyncState(ctx context.Context, sourceID string) (*domain.SyncState, error) {
	// First verify the source exists
	_, err := o.sourceStore.Get(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	// Get sync state from store
	state, err := o.syncStore.Get(ctx, sourceID)
	if err != nil {
		if err == domain.ErrNotFound {
			// Return empty state if none exists
			return &domain.SyncState{
				SourceID: sourceID,
				Status:   domain.SyncStatusIdle,
				Stats:    domain.SyncStats{},
			}, nil
		}
		return nil, err
	}

	return state, nil
}

// ListSyncStates retrieves sync states for all sources.
func (o *SyncOrchestrator) ListSyncStates(ctx context.Context) ([]*domain.SyncState, error) {
	// Get all sources
	sources, err := o.sourceStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	states := make([]*domain.SyncState, 0, len(sources))
	for _, source := range sources {
		state, err := o.GetSyncState(ctx, source.ID)
		if err != nil {
			o.logger.Warn("failed to get sync state", "source_id", source.ID, "error", err)
			// Include a placeholder state for sources where we couldn't get state
			states = append(states, &domain.SyncState{
				SourceID: source.ID,
				Status:   domain.SyncStatusIdle,
				Error:    err.Error(),
			})
			continue
		}
		states = append(states, state)
	}

	return states, nil
}

// CancelSync cancels an ongoing sync for a source.
// Note: This is a placeholder - actual cancellation requires context propagation.
func (o *SyncOrchestrator) CancelSync(ctx context.Context, sourceID string) error {
	// Get current sync state
	state, err := o.syncStore.Get(ctx, sourceID)
	if err != nil {
		return err
	}

	// Only running syncs can be cancelled
	if state.Status != domain.SyncStatusRunning {
		return nil
	}

	// Mark as failed/cancelled
	now := time.Now()
	state.Status = domain.SyncStatusFailed
	state.CompletedAt = &now
	state.Error = "cancelled by user"

	return o.syncStore.Save(ctx, state)
}

// loadSettings loads team settings for the sync orchestrator
func (o *SyncOrchestrator) loadSettings(ctx context.Context) (*domain.Settings, error) {
	if o.settingsStore == nil {
		return domain.DefaultSettings(o.teamID), nil
	}
	return o.settingsStore.GetSettings(ctx, o.teamID)
}

// shouldExclude checks if a document should be excluded based on sync exclusion patterns
func (o *SyncOrchestrator) shouldExclude(ctx context.Context, doc *domain.Document) bool {
	settings, err := o.loadSettings(ctx)
	if err != nil || settings.SyncExclusions == nil || !settings.SyncExclusions.HasPatterns() {
		return false
	}

	activePatterns := settings.SyncExclusions.GetActivePatterns()
	return o.matchesExclusionPattern(doc.Path, activePatterns)
}

// matchesExclusionPattern checks if a path matches any exclusion pattern
func (o *SyncOrchestrator) matchesExclusionPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if o.matchPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchPattern matches a path against a pattern
// Supports:
// - Exact matches
// - Glob patterns (*.txt, *.log)
// - Folder patterns (.git/, node_modules/)
// - Prefix matching for folder patterns
func (o *SyncOrchestrator) matchPattern(path, pattern string) bool {
	// Handle folder patterns (ending with /)
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		// Prefix match for folder patterns
		// Check if path starts with pattern or contains it as a path component
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
		// Check if pattern appears as a path component
		// e.g., pattern ".git/" matches "foo/.git/bar"
		if len(path) > len(pattern) {
			for i := 0; i < len(path)-len(pattern); i++ {
				if path[i] == '/' && path[i+1:i+1+len(pattern)] == pattern {
					return true
				}
			}
		}
		return false
	}

	// Handle exact filename matches (e.g., ".DS_Store", "Thumbs.db")
	// Extract filename from path
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}
	filename := path
	if lastSlash >= 0 {
		filename = path[lastSlash+1:]
	}

	// Exact match on filename
	if filename == pattern {
		return true
	}

	// Glob pattern matching (*.txt, *.log, etc)
	if len(pattern) > 2 && pattern[0] == '*' && pattern[1] == '.' {
		// Extract extension from pattern
		ext := pattern[1:] // e.g., ".txt"
		// Check if filename ends with this extension
		if len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}

	return false
}
