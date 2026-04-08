package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
	"github.com/sercha-oss/sercha-core/internal/runtime"
)

// We need a ChunkStore for saving chunks separately
// The SyncOrchestrator needs both DocumentStore and ChunkStore

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
	sourceStore      driven.SourceStore
	documentStore    driven.DocumentStore
	chunkStore       driven.ChunkStore
	syncStore        driven.SyncStateStore
	searchEngine     driven.SearchEngine
	vectorIndex      driven.VectorIndex
	connectorFactory driven.ConnectorFactory
	normaliserReg    driven.NormaliserRegistry
	services         *runtime.Services
	logger           *slog.Logger
	indexingExecutor pipelineport.IndexingExecutor // Required pipeline executor
	capabilitySet    *pipeline.CapabilitySet       // Capabilities for pipeline
	capabilityStore  driven.CapabilityStore        // For fetching capability preferences
	settingsStore    driven.SettingsStore          // For loading team settings
	teamID           string                        // Team ID for settings lookup
}

// SyncOrchestratorConfig holds dependencies for SyncOrchestrator.
type SyncOrchestratorConfig struct {
	SourceStore      driven.SourceStore
	DocumentStore    driven.DocumentStore
	ChunkStore       driven.ChunkStore
	SyncStore        driven.SyncStateStore
	SearchEngine     driven.SearchEngine
	VectorIndex      driven.VectorIndex
	ConnectorFactory driven.ConnectorFactory
	NormaliserReg    driven.NormaliserRegistry
	Services         *runtime.Services
	Logger           *slog.Logger
	IndexingExecutor pipelineport.IndexingExecutor // Required pipeline executor
	CapabilitySet    *pipeline.CapabilitySet       // Capabilities for pipeline
	CapabilityStore  driven.CapabilityStore        // For fetching capability preferences
	SettingsStore    driven.SettingsStore          // For loading team settings
	TeamID           string                        // Team ID for settings lookup
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

	return &SyncOrchestrator{
		sourceStore:      cfg.SourceStore,
		documentStore:    cfg.DocumentStore,
		chunkStore:       cfg.ChunkStore,
		syncStore:        cfg.SyncStore,
		searchEngine:     cfg.SearchEngine,
		vectorIndex:      cfg.VectorIndex,
		connectorFactory: cfg.ConnectorFactory,
		normaliserReg:    cfg.NormaliserReg,
		services:         cfg.Services,
		logger:           logger,
		indexingExecutor: cfg.IndexingExecutor,
		capabilitySet:    cfg.CapabilitySet,
		capabilityStore:  cfg.CapabilityStore,
		settingsStore:    cfg.SettingsStore,
		teamID:           cfg.TeamID,
	}
}

// SyncSource synchronizes a single source.
// This is the main entry point for the sync pipeline.
// For sources with container selection, it syncs each selected container.
func (o *SyncOrchestrator) SyncSource(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
	startTime := time.Now()

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

	// Step 3: Sync each container
	for _, containerID := range containerIDs {
		containerStats, cursor, err := o.syncContainer(ctx, source, syncState, containerID)
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
func (o *SyncOrchestrator) syncContainer(
	ctx context.Context,
	source *domain.Source,
	syncState *domain.SyncState,
	containerID string,
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

	// Full sync (no cursor): wipe all existing indexed data for this source
	// to prevent orphaned chunks from accumulating across re-syncs.
	if cursor == "" {
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

	for {
		select {
		case <-ctx.Done():
			return stats, lastCursor, ctx.Err()
		default:
		}

		changes, nextCursor, err := connector.FetchChanges(ctx, source, cursor)
		if err != nil {
			return stats, lastCursor, fmt.Errorf("failed to fetch changes: %w", err)
		}

		if len(changes) == 0 {
			break
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

		// Process each document
		errorsBefore := stats.Errors
		for _, change := range changes {
			if err := o.processChange(ctx, source, change, stats); err != nil {
				o.logger.Warn("failed to process change",
					"source_id", source.ID,
					"container_id", containerID,
					"external_id", change.ExternalID,
					"error", err,
				)
				stats.Errors++
			}
		}

		// Only advance cursor if all documents in this batch succeeded
		if stats.Errors == errorsBefore {
			lastCursor = nextCursor
		} else {
			o.logger.Warn("not advancing cursor due to failed documents",
				"source_id", source.ID,
				"container_id", containerID,
				"failed_count", stats.Errors-errorsBefore,
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
		return o.processDelete(ctx, source.ID, change, stats)
	case domain.ChangeTypeAdded, domain.ChangeTypeModified:
		return o.processAddOrUpdate(ctx, source, change, stats)
	default:
		return fmt.Errorf("unknown change type: %s", change.Type)
	}
}

// processDelete handles document deletion across all storage layers.
func (o *SyncOrchestrator) processDelete(
	ctx context.Context,
	sourceID string,
	change *domain.Change,
	stats *domain.SyncStats,
) error {
	doc, err := o.documentStore.GetByExternalID(ctx, sourceID, change.ExternalID)
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

	stats.DocumentsDeleted++
	return nil
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

	// Update stats
	if isUpdate {
		stats.DocumentsUpdated++
	} else {
		stats.DocumentsAdded++
	}
	stats.ChunksIndexed += len(output.ChunkIDs)

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
