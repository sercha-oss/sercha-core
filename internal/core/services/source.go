package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure sourceService implements SourceService
var _ driving.SourceService = (*sourceService)(nil)

// sourceService implements the SourceService interface
type sourceService struct {
	sourceStore   driven.SourceStore
	documentStore driven.DocumentStore
	syncStore     driven.SyncStateStore
	searchEngine  driven.SearchEngine
	vectorIndex   driven.VectorIndex
	taskQueue     driven.TaskQueue
	teamID        string
	logger        *slog.Logger
}

// NewSourceService creates a new SourceService
func NewSourceService(
	sourceStore driven.SourceStore,
	documentStore driven.DocumentStore,
	syncStore driven.SyncStateStore,
	searchEngine driven.SearchEngine,
	vectorIndex driven.VectorIndex,
	taskQueue driven.TaskQueue,
	teamID string,
	logger *slog.Logger,
) driving.SourceService {
	if logger == nil {
		logger = slog.Default()
	}
	return &sourceService{
		sourceStore:   sourceStore,
		documentStore: documentStore,
		syncStore:     syncStore,
		searchEngine:  searchEngine,
		vectorIndex:   vectorIndex,
		taskQueue:     taskQueue,
		teamID:        teamID,
		logger:        logger,
	}
}

// Create creates a new source (admin only)
func (s *sourceService) Create(ctx context.Context, creatorID string, req driving.CreateSourceRequest) (*domain.Source, error) {
	// Validate input
	if req.Name == "" {
		return nil, domain.ErrInvalidInput
	}

	// Check if name already exists
	existing, _ := s.sourceStore.GetByName(ctx, req.Name)
	if existing != nil {
		return nil, domain.ErrAlreadyExists
	}

	now := time.Now()
	source := &domain.Source{
		ID:           generateID(),
		Name:         strings.TrimSpace(req.Name),
		ProviderType: req.ProviderType,
		Config:       req.Config,
		ConnectionID: req.ConnectionID,
		Containers:   req.Containers,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    creatorID,
	}

	if err := s.sourceStore.Save(ctx, source); err != nil {
		return nil, err
	}

	// Initialize sync state
	syncState := &domain.SyncState{
		SourceID: source.ID,
		Status:   domain.SyncStatusIdle,
		Stats:    domain.SyncStats{},
	}
	_ = s.syncStore.Save(ctx, syncState)

	return source, nil
}

// Get retrieves a source by ID
func (s *sourceService) Get(ctx context.Context, id string) (*domain.Source, error) {
	return s.sourceStore.Get(ctx, id)
}

// List retrieves all sources
func (s *sourceService) List(ctx context.Context) ([]*domain.Source, error) {
	return s.sourceStore.List(ctx)
}

// ListByConnection retrieves all sources using a specific connection
func (s *sourceService) ListByConnection(ctx context.Context, connectionID string) ([]*domain.Source, error) {
	return s.sourceStore.ListByConnection(ctx, connectionID)
}

// ListWithSummary retrieves all sources with document counts
func (s *sourceService) ListWithSummary(ctx context.Context) ([]*domain.SourceSummary, error) {
	sources, err := s.sourceStore.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]*domain.SourceSummary, 0, len(sources))
	for _, source := range sources {
		count, _ := s.documentStore.CountBySource(ctx, source.ID)
		syncState, _ := s.syncStore.Get(ctx, source.ID)

		summary := &domain.SourceSummary{
			Source:        source,
			DocumentCount: count,
			SyncStatus:    string(domain.SyncStatusIdle),
		}

		if syncState != nil {
			summary.LastSyncAt = syncState.LastSyncAt
			summary.SyncStatus = string(syncState.Status)
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// Update updates a source (admin only)
func (s *sourceService) Update(ctx context.Context, id string, req driving.UpdateSourceRequest) (*domain.Source, error) {
	source, err := s.sourceStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		// Check if new name conflicts with existing source
		if *req.Name != source.Name {
			existing, _ := s.sourceStore.GetByName(ctx, *req.Name)
			if existing != nil && existing.ID != id {
				return nil, domain.ErrAlreadyExists
			}
		}
		source.Name = strings.TrimSpace(*req.Name)
	}

	if req.Config != nil {
		source.Config = *req.Config
	}

	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}

	source.UpdatedAt = time.Now()

	if err := s.sourceStore.Save(ctx, source); err != nil {
		return nil, err
	}

	return source, nil
}

// Delete deletes a source and all its documents (admin only)
func (s *sourceService) Delete(ctx context.Context, id string) error {
	// Verify source exists
	_, err := s.sourceStore.Get(ctx, id)
	if err != nil {
		return err
	}

	// Delete from search engine first
	if s.searchEngine != nil {
		_ = s.searchEngine.DeleteBySource(ctx, id)
	}

	// Delete documents
	if err := s.documentStore.DeleteBySource(ctx, id); err != nil {
		return err
	}

	// Delete sync state
	_ = s.syncStore.Delete(ctx, id)

	// Delete source
	return s.sourceStore.Delete(ctx, id)
}

// Enable enables a source
func (s *sourceService) Enable(ctx context.Context, id string) error {
	return s.sourceStore.SetEnabled(ctx, id, true)
}

// Disable disables a source
func (s *sourceService) Disable(ctx context.Context, id string) error {
	return s.sourceStore.SetEnabled(ctx, id, false)
}

// UpdateContainers updates the selected containers for a source
func (s *sourceService) UpdateContainers(ctx context.Context, id string, containers []domain.Container) error {
	// Get existing source with current containers
	source, err := s.sourceStore.Get(ctx, id)
	if err != nil {
		return err
	}

	// Build maps for efficient comparison
	oldContainerMap := make(map[string]domain.Container)
	for _, c := range source.Containers {
		oldContainerMap[c.ID] = c
	}

	newContainerMap := make(map[string]domain.Container)
	for _, c := range containers {
		newContainerMap[c.ID] = c
	}

	// Identify removed and added containers
	var removedContainers []domain.Container
	var addedContainers []domain.Container

	// Find removed containers (in old but not in new)
	for id, container := range oldContainerMap {
		if _, exists := newContainerMap[id]; !exists {
			removedContainers = append(removedContainers, container)
		}
	}

	// Find added containers (in new but not in old)
	for id, container := range newContainerMap {
		if _, exists := oldContainerMap[id]; !exists {
			addedContainers = append(addedContainers, container)
		}
	}

	// Delete indexed data for removed containers
	for _, container := range removedContainers {
		if err := s.deleteContainerData(ctx, source.ID, container.ID); err != nil {
			// Log error but continue with other containers
			// We don't want to fail the entire operation if one deletion fails
			s.logger.Error("failed to delete container data",
				"source_id", source.ID,
				"container_id", container.ID,
				"error", err)
		}
	}

	// Enqueue sync tasks for added containers
	for _, container := range addedContainers {
		if s.taskQueue != nil {
			task := domain.NewSyncContainerTask(s.teamID, source.ID, container.ID)
			if err := s.taskQueue.Enqueue(ctx, task); err != nil {
				// Log error but continue
				// The container will be synced on next full sync if this fails
				s.logger.Error("failed to enqueue sync task for container",
					"source_id", source.ID,
					"container_id", container.ID,
					"error", err)
			}
		}
	}

	// Update containers in store
	return s.sourceStore.UpdateContainers(ctx, id, containers)
}

// deleteContainerData deletes all indexed data for a specific container
func (s *sourceService) deleteContainerData(ctx context.Context, sourceID, containerID string) error {
	// Delete from search engine (OpenSearch)
	if s.searchEngine != nil {
		if err := s.searchEngine.DeleteBySourceAndContainer(ctx, sourceID, containerID); err != nil {
			return err
		}
	}

	// Delete from vector index (pgvector)
	if s.vectorIndex != nil {
		if err := s.vectorIndex.DeleteBySourceAndContainer(ctx, sourceID, containerID); err != nil {
			return err
		}
	}

	// Delete documents (PostgreSQL)
	if s.documentStore != nil {
		if err := s.documentStore.DeleteBySourceAndContainer(ctx, sourceID, containerID); err != nil {
			return err
		}
	}

	return nil
}
