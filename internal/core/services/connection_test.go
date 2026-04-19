package services

import (
	"context"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
)

func TestConnectionService_List(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore: connStore,
		SourceStore:     sourceStore,
	})

	// Save some connections
	now := time.Now()
	conn1 := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		AccountID:    "user1",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	conn2 := &domain.Connection{
		ID:           "conn-2",
		Name:         "Test LocalFS",
		Platform:     domain.PlatformLocalFS,
		ProviderType: domain.ProviderTypeLocalFS,
		AuthMethod:   domain.AuthMethodOAuth2,
		AccountID:    "user2",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	ctx := context.Background()
	_ = connStore.Save(ctx, conn1)
	_ = connStore.Save(ctx, conn2)

	// Test List
	summaries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("List() got %d connections, want 2", len(summaries))
	}
}

func TestConnectionService_Get(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore: connStore,
		SourceStore:     sourceStore,
	})

	ctx := context.Background()

	// Test not found
	_, err := svc.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get() expected error for nonexistent connection")
	}

	// Save a connection
	now := time.Now()
	conn := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		AccountID:    "user1",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = connStore.Save(ctx, conn)

	// Test found
	summary, err := svc.Get(ctx, "conn-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if summary.ID != "conn-1" {
		t.Errorf("Get() got ID = %s, want conn-1", summary.ID)
	}
	if summary.Name != "Test GitHub" {
		t.Errorf("Get() got Name = %s, want Test GitHub", summary.Name)
	}
}

func TestConnectionService_Delete(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore: connStore,
		SourceStore:     sourceStore,
	})

	ctx := context.Background()

	// Save a connection
	now := time.Now()
	conn := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		AccountID:    "user1",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = connStore.Save(ctx, conn)

	// Delete should succeed
	err := svc.Delete(ctx, "conn-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	if connStore.Count() != 0 {
		t.Error("Delete() connection still exists")
	}
}

func TestConnectionService_Delete_InUse(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore: connStore,
		SourceStore:     sourceStore,
	})

	ctx := context.Background()

	// Save a connection
	now := time.Now()
	conn := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		AccountID:    "user1",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = connStore.Save(ctx, conn)

	// Save a source using this connection
	source := &domain.Source{
		ID:           "src-1",
		Name:         "My Repos",
		ProviderType: domain.ProviderTypeGitHub,
		ConnectionID: "conn-1",
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = sourceStore.Save(ctx, source)

	// Delete should fail with ErrInUse
	err := svc.Delete(ctx, "conn-1")
	if err != domain.ErrInUse {
		t.Errorf("Delete() error = %v, want ErrInUse", err)
	}
}

func TestConnectionService_ListByProvider(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore: connStore,
		SourceStore:     sourceStore,
	})

	ctx := context.Background()

	// Save connections for different providers
	now := time.Now()
	conn1 := &domain.Connection{
		ID:           "conn-1",
		Name:         "GitHub 1",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	conn2 := &domain.Connection{
		ID:           "conn-2",
		Name:         "GitHub 2",
		Platform:     domain.PlatformGitHub,
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	conn3 := &domain.Connection{
		ID:           "conn-3",
		Name:         "LocalFS",
		Platform:     domain.PlatformLocalFS,
		ProviderType: domain.ProviderTypeLocalFS,
		AuthMethod:   domain.AuthMethodOAuth2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_ = connStore.Save(ctx, conn1)
	_ = connStore.Save(ctx, conn2)
	_ = connStore.Save(ctx, conn3)

	// List by GitHub
	summaries, err := svc.ListByProvider(ctx, domain.ProviderTypeGitHub)
	if err != nil {
		t.Fatalf("ListByProvider() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("ListByProvider() got %d connections, want 2", len(summaries))
	}

	// List by LocalFS
	summaries, err = svc.ListByProvider(ctx, domain.ProviderTypeLocalFS)
	if err != nil {
		t.Fatalf("ListByProvider() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("ListByProvider() got %d connections, want 1", len(summaries))
	}
}

// MockContainerListerFactory is a mock for testing
type mockContainerListerFactory struct {
	lister              driven.ContainerLister
	supportsContainerFn func(domain.ProviderType) bool
}

func (m *mockContainerListerFactory) Create(ctx context.Context, providerType domain.ProviderType, connectionID string) (driven.ContainerLister, error) {
	return m.lister, nil
}

func (m *mockContainerListerFactory) SupportsContainerSelection(providerType domain.ProviderType) bool {
	if m.supportsContainerFn != nil {
		return m.supportsContainerFn(providerType)
	}
	return true
}

// mockContainerLister for testing
type mockContainerLister struct {
	containers []*driven.Container
	nextCursor string
}

func (m *mockContainerLister) ListContainers(ctx context.Context, cursor string, parentID string) ([]*driven.Container, string, error) {
	return m.containers, m.nextCursor, nil
}

func TestConnectionService_ListContainers(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	// Create mock container lister
	containers := []*driven.Container{
		{ID: "owner/repo1", Name: "repo1", Type: "repository"},
		{ID: "owner/repo2", Name: "repo2", Type: "repository"},
	}
	lister := &mockContainerLister{containers: containers}
	factory := &mockContainerListerFactory{lister: lister}

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore:        connStore,
		SourceStore:            sourceStore,
		ContainerListerFactory: factory,
	})

	ctx := context.Background()

	// Save a connection
	now := time.Now()
	conn := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = connStore.Save(ctx, conn)

	// List containers
	resp, err := svc.ListContainers(ctx, "conn-1", "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}
	if len(resp.Containers) != 2 {
		t.Errorf("ListContainers() got %d containers, want 2", len(resp.Containers))
	}
}

func TestConnectionService_ListContainers_UnsupportedProvider(t *testing.T) {
	connStore := mocks.NewMockConnectionStore()
	sourceStore := mocks.NewMockSourceStore()

	// Create factory that doesn't support container selection for this provider
	factory := &mockContainerListerFactory{
		supportsContainerFn: func(pt domain.ProviderType) bool {
			return false
		},
	}

	svc := NewConnectionService(ConnectionServiceConfig{
		ConnectionStore:        connStore,
		SourceStore:            sourceStore,
		ContainerListerFactory: factory,
	})

	ctx := context.Background()

	// Save a connection
	now := time.Now()
	conn := &domain.Connection{
		ID:           "conn-1",
		Name:         "Test GitHub",
		ProviderType: domain.ProviderTypeGitHub,
		AuthMethod:   domain.AuthMethodOAuth2,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = connStore.Save(ctx, conn)

	// List containers should return empty list
	resp, err := svc.ListContainers(ctx, "conn-1", "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}
	if len(resp.Containers) != 0 {
		t.Errorf("ListContainers() got %d containers, want 0 for unsupported provider", len(resp.Containers))
	}
}
