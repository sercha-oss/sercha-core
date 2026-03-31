package services

import (
	"context"
	"strings"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Ensure sourceService implements SourceService
var _ driving.SourceService = (*sourceService)(nil)

// sourceService implements the SourceService interface
type sourceService struct {
	sourceStore   driven.SourceStore
	documentStore driven.DocumentStore
	syncStore     driven.SyncStateStore
	searchEngine  driven.SearchEngine
}

// NewSourceService creates a new SourceService
func NewSourceService(
	sourceStore driven.SourceStore,
	documentStore driven.DocumentStore,
	syncStore driven.SyncStateStore,
	searchEngine driven.SearchEngine,
) driving.SourceService {
	return &sourceService{
		sourceStore:   sourceStore,
		documentStore: documentStore,
		syncStore:     syncStore,
		searchEngine:  searchEngine,
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
	// Verify source exists
	_, err := s.sourceStore.Get(ctx, id)
	if err != nil {
		return err
	}

	return s.sourceStore.UpdateContainers(ctx, id, containers)
}
