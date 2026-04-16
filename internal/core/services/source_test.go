package services

import (
	"context"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

func TestSourceService_Create(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	vectorIndex := mocks.NewMockVectorIndex()
	taskQueue := mocks.NewMockTaskQueue()
	teamID := "test-team"
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, vectorIndex, taskQueue, teamID, nil)

	tests := []struct {
		name      string
		creatorID string
		req       driving.CreateSourceRequest
		wantErr   error
	}{
		{
			name:      "valid source",
			creatorID: "user-123",
			req: driving.CreateSourceRequest{
				Name:         "Test Source",
				ProviderType: domain.ProviderTypeGitHub,
				Config: domain.SourceConfig{
					Owner:      "test-org",
					Repository: "test-repo",
				},
			},
			wantErr: nil,
		},
		{
			name:      "missing name",
			creatorID: "user-123",
			req: driving.CreateSourceRequest{
				Name:         "",
				ProviderType: domain.ProviderTypeGitHub,
			},
			wantErr: domain.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := svc.Create(context.Background(), tt.creatorID, tt.req)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if source == nil {
				t.Fatal("expected source to be returned")
			}
			if source.Name != tt.req.Name {
				t.Errorf("expected name %s, got %s", tt.req.Name, source.Name)
			}
			if source.ProviderType != tt.req.ProviderType {
				t.Errorf("expected provider type %s, got %s", tt.req.ProviderType, source.ProviderType)
			}
			if source.CreatedBy != tt.creatorID {
				t.Errorf("expected created by %s, got %s", tt.creatorID, source.CreatedBy)
			}
			if !source.Enabled {
				t.Error("expected source to be enabled")
			}
		})
	}
}

func TestSourceService_Create_DuplicateName(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	req := driving.CreateSourceRequest{
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
	}

	// Create first source
	_, err := svc.Create(context.Background(), "user-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to create duplicate
	_, err = svc.Create(context.Background(), "user-123", req)
	if err != domain.ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestSourceService_Get(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source
	source := &domain.Source{
		ID:           "source-123",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(context.Background(), source)

	// Get the source
	result, err := svc.Get(context.Background(), "source-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != source.ID {
		t.Errorf("expected source ID %s, got %s", source.ID, result.ID)
	}

	// Get non-existent source
	_, err = svc.Get(context.Background(), "non-existent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSourceService_List(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create sources
	for i := 0; i < 3; i++ {
		source := &domain.Source{
			ID:           generateID(),
			Name:         "Source " + string(rune('A'+i)),
			ProviderType: domain.ProviderTypeGitHub,
			Enabled:      true,
		}
		_ = sourceStore.Save(context.Background(), source)
	}

	// List sources
	sources, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(sources))
	}
}

func TestSourceService_ListWithSummary(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source
	source := &domain.Source{
		ID:           "source-123",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
	}
	_ = sourceStore.Save(context.Background(), source)

	// Add documents
	for i := 0; i < 5; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-123",
			Title:    "Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Add sync state
	now := time.Now()
	syncState := &domain.SyncState{
		SourceID:   "source-123",
		Status:     domain.SyncStatusCompleted,
		LastSyncAt: &now,
	}
	_ = syncStore.Save(context.Background(), syncState)

	// List with summary
	summaries, err := svc.ListWithSummary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	summary := summaries[0]
	if summary.Source.ID != "source-123" {
		t.Errorf("expected source ID source-123, got %s", summary.Source.ID)
	}
	if summary.DocumentCount != 5 {
		t.Errorf("expected 5 documents, got %d", summary.DocumentCount)
	}
	if summary.SyncStatus != "completed" {
		t.Errorf("expected sync status completed, got %s", summary.SyncStatus)
	}
	if summary.LastSyncAt == nil {
		t.Error("expected LastSyncAt to be set")
	}
}

func TestSourceService_Update(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source
	source := &domain.Source{
		ID:           "source-123",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Config: domain.SourceConfig{
			Owner:      "test-org",
			Repository: "test-repo",
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	// Update name
	newName := "Updated Source"
	updated, err := svc.Update(context.Background(), "source-123", driving.UpdateSourceRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected name %s, got %s", newName, updated.Name)
	}

	// Update enabled
	enabled := false
	updated, err = svc.Update(context.Background(), "source-123", driving.UpdateSourceRequest{
		Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Enabled != enabled {
		t.Errorf("expected enabled %v, got %v", enabled, updated.Enabled)
	}
}

func TestSourceService_Update_ConflictingName(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create two sources
	source1 := &domain.Source{
		ID:   "source-1",
		Name: "Source One",
	}
	_ = sourceStore.Save(context.Background(), source1)

	source2 := &domain.Source{
		ID:   "source-2",
		Name: "Source Two",
	}
	_ = sourceStore.Save(context.Background(), source2)

	// Try to update source2 to have source1's name
	conflictingName := "Source One"
	_, err := svc.Update(context.Background(), "source-2", driving.UpdateSourceRequest{
		Name: &conflictingName,
	})
	if err != domain.ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestSourceService_Delete(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source with documents and chunks
	source := &domain.Source{
		ID:   "source-123",
		Name: "Test Source",
	}
	_ = sourceStore.Save(context.Background(), source)

	doc := &domain.Document{
		ID:       "doc-123",
		SourceID: "source-123",
	}
	_ = documentStore.Save(context.Background(), doc)

	chunk := &domain.Chunk{
		ID:         "chunk-123",
		DocumentID: "doc-123",
		SourceID:   "source-123",
		Content:    "Test content",
	}
	_ = searchEngine.Index(context.Background(), []*domain.Chunk{chunk})

	syncState := &domain.SyncState{
		SourceID: "source-123",
		Status:   domain.SyncStatusIdle,
	}
	_ = syncStore.Save(context.Background(), syncState)

	// Delete the source
	err := svc.Delete(context.Background(), "source-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify source is deleted
	_, err = svc.Get(context.Background(), "source-123")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}

	// Verify documents are deleted
	count, _ := documentStore.CountBySource(context.Background(), "source-123")
	if count != 0 {
		t.Errorf("expected 0 documents, got %d", count)
	}
}

func TestSourceService_EnableDisable(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source
	source := &domain.Source{
		ID:      "source-123",
		Name:    "Test Source",
		Enabled: true,
	}
	_ = sourceStore.Save(context.Background(), source)

	// Disable
	err := svc.Disable(context.Background(), "source-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, _ := svc.Get(context.Background(), "source-123")
	if result.Enabled {
		t.Error("expected source to be disabled")
	}

	// Enable
	err = svc.Enable(context.Background(), "source-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, _ = svc.Get(context.Background(), "source-123")
	if !result.Enabled {
		t.Error("expected source to be enabled")
	}
}

func TestSourceService_ListByConnection(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create sources with different connections
	source1 := &domain.Source{
		ID:           "source-1",
		Name:         "Source 1",
		ProviderType: domain.ProviderTypeGitHub,
		ConnectionID: "conn-1",
		Enabled:      true,
	}
	_ = sourceStore.Save(context.Background(), source1)

	source2 := &domain.Source{
		ID:           "source-2",
		Name:         "Source 2",
		ProviderType: domain.ProviderTypeGitHub,
		ConnectionID: "conn-1",
		Enabled:      true,
	}
	_ = sourceStore.Save(context.Background(), source2)

	source3 := &domain.Source{
		ID:           "source-3",
		Name:         "Source 3",
		ProviderType: domain.ProviderTypeGitHub,
		ConnectionID: "conn-2",
		Enabled:      true,
	}
	_ = sourceStore.Save(context.Background(), source3)

	// List sources for connection 1
	sources, err := svc.ListByConnection(context.Background(), "conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("expected 2 sources for conn-1, got %d", len(sources))
	}

	// Verify correct sources are returned
	foundSource1 := false
	foundSource2 := false
	for _, s := range sources {
		if s.ID == "source-1" {
			foundSource1 = true
		}
		if s.ID == "source-2" {
			foundSource2 = true
		}
		if s.ConnectionID != "conn-1" {
			t.Errorf("expected connection ID conn-1, got %s", s.ConnectionID)
		}
	}
	if !foundSource1 || !foundSource2 {
		t.Error("expected to find both source-1 and source-2")
	}

	// List sources for connection 2
	sources, err = svc.ListByConnection(context.Background(), "conn-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source for conn-2, got %d", len(sources))
	}
	if sources[0].ID != "source-3" {
		t.Errorf("expected source-3, got %s", sources[0].ID)
	}

	// List sources for non-existent connection
	sources, err = svc.ListByConnection(context.Background(), "conn-999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources for conn-999, got %d", len(sources))
	}
}

func TestSourceService_UpdateContainers(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Create a source
	source := &domain.Source{
		ID:           "source-123",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		Containers:   []domain.Container{},
	}
	_ = sourceStore.Save(context.Background(), source)

	// Update containers
	newContainers := []domain.Container{
		{ID: "repo-1", Name: "org/repo1", Type: "repository"},
		{ID: "repo-2", Name: "org/repo2", Type: "repository"},
	}
	err := svc.UpdateContainers(context.Background(), "source-123", newContainers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify containers were updated
	result, _ := svc.Get(context.Background(), "source-123")
	if len(result.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(result.Containers))
	}
	if result.Containers[0].ID != "repo-1" {
		t.Errorf("expected container ID repo-1, got %s", result.Containers[0].ID)
	}
	if result.Containers[1].Name != "org/repo2" {
		t.Errorf("expected container name org/repo2, got %s", result.Containers[1].Name)
	}
}

func TestSourceService_UpdateContainers_NotFound(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	documentStore := mocks.NewMockDocumentStore()
	syncStore := mocks.NewMockSyncStateStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewSourceService(sourceStore, documentStore, syncStore, searchEngine, mocks.NewMockVectorIndex(), mocks.NewMockTaskQueue(), "test-team", nil)

	// Try to update containers for non-existent source
	containers := []domain.Container{
		{ID: "repo-1", Name: "org/repo1", Type: "repository"},
	}
	err := svc.UpdateContainers(context.Background(), "non-existent", containers)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
