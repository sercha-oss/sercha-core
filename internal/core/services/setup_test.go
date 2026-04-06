package services

import (
	"context"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven/mocks"
)

func newTestSetupService() (*mocks.MockUserStore, *mocks.MockSourceStore, *mocks.MockVespaConfigStore, *setupService) {
	userStore := mocks.NewMockUserStore()
	sourceStore := mocks.NewMockSourceStore()
	vespaConfigStore := mocks.NewMockVespaConfigStore()
	svc := NewSetupService(userStore, sourceStore, vespaConfigStore, "team-123").(*setupService)
	return userStore, sourceStore, vespaConfigStore, svc
}

func TestSetupService_GetStatus_SetupIncomplete_NoUsers(t *testing.T) {
	_, sourceStore, _, svc := newTestSetupService()

	// No users exist (empty store)

	// Sources exist
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status == nil {
		t.Fatal("expected status to be returned")
	}

	if status.SetupComplete {
		t.Error("expected setup to be incomplete when no users exist")
	}

	if status.HasUsers {
		t.Error("expected HasUsers to be false")
	}

	if !status.HasSources {
		t.Error("expected HasSources to be true")
	}
}

func TestSetupService_GetStatus_SetupIncomplete_NoSources(t *testing.T) {
	userStore, _, _, svc := newTestSetupService()

	// Users exist
	user := &domain.User{
		ID:        "user-1",
		Email:     "test@example.com",
		Name:      "Test User",
		TeamID:    "team-123",
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	// No sources exist (empty store)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Setup is complete once users exist (sources are configured separately)
	if !status.SetupComplete {
		t.Error("expected setup to be complete when users exist (sources configured separately)")
	}

	if !status.HasUsers {
		t.Error("expected HasUsers to be true")
	}

	if status.HasSources {
		t.Error("expected HasSources to be false")
	}
}

func TestSetupService_GetStatus_SetupComplete(t *testing.T) {
	userStore, sourceStore, _, svc := newTestSetupService()

	// Users exist
	user := &domain.User{
		ID:        "user-1",
		Email:     "admin@example.com",
		Name:      "Admin User",
		TeamID:    "team-123",
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	// Sources exist
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.SetupComplete {
		t.Error("expected setup to be complete when users exist")
	}

	if !status.HasUsers {
		t.Error("expected HasUsers to be true")
	}

	if !status.HasSources {
		t.Error("expected HasSources to be true")
	}
}

func TestSetupService_GetStatus_VespaConnected(t *testing.T) {
	userStore := mocks.NewMockUserStore()
	sourceStore := mocks.NewMockSourceStore()
	vespaConfigStore := mocks.NewMockVespaConfigStore()
	svc := NewSetupService(userStore, sourceStore, vespaConfigStore, "team-123").(*setupService)

	// Setup basic data
	user := &domain.User{
		ID:        "user-1",
		Email:     "test@example.com",
		TeamID:    "team-123",
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	// Vespa is connected
	vespaConfigStore.GetVespaConfigFunc = func(ctx context.Context, teamID string) (*domain.VespaConfig, error) {
		return &domain.VespaConfig{
			TeamID:     teamID,
			Endpoint:   "http://vespa:8080",
			DevMode:    true,
			Connected:  true,
			SchemaMode: domain.VespacSchemaModeBM25,
		}, nil
	}

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.VespaConnected {
		t.Error("expected VespaConnected to be true")
	}
}

func TestSetupService_GetStatus_VespaNotConfigured(t *testing.T) {
	userStore := mocks.NewMockUserStore()
	sourceStore := mocks.NewMockSourceStore()
	vespaConfigStore := mocks.NewMockVespaConfigStore()
	svc := NewSetupService(userStore, sourceStore, vespaConfigStore, "team-123").(*setupService)

	// Setup basic data
	user := &domain.User{
		ID:        "user-1",
		Email:     "test@example.com",
		TeamID:    "team-123",
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	// Vespa not configured (default mock behavior returns ErrNotFound)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.VespaConnected {
		t.Error("expected VespaConnected to be false when Vespa is not configured")
	}
}

func TestSetupService_GetStatus_EmptyState(t *testing.T) {
	_, _, _, svc := newTestSetupService()

	// No users, no sources (empty stores)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.SetupComplete {
		t.Error("expected setup to be incomplete in empty state")
	}

	if status.HasUsers {
		t.Error("expected HasUsers to be false")
	}

	if status.HasSources {
		t.Error("expected HasSources to be false")
	}

	if status.VespaConnected {
		t.Error("expected VespaConnected to be false")
	}
}

func TestSetupService_GetStatus_MultipleUsers(t *testing.T) {
	userStore, sourceStore, _, svc := newTestSetupService()

	// Multiple users exist
	for i := 1; i <= 3; i++ {
		user := &domain.User{
			ID:        domain.GenerateID(),
			Email:     "user" + string(rune('0'+i)) + "@example.com",
			Name:      "User",
			TeamID:    "team-123",
			Role:      domain.RoleMember,
			Active:    true,
			CreatedAt: time.Now(),
		}
		_ = userStore.Save(context.Background(), user)
	}

	// Sources exist
	source := &domain.Source{
		ID:           "source-1",
		Name:         "Test Source",
		ProviderType: domain.ProviderTypeGitHub,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	_ = sourceStore.Save(context.Background(), source)

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.SetupComplete {
		t.Error("expected setup to be complete with multiple users and sources")
	}

	if !status.HasUsers {
		t.Error("expected HasUsers to be true")
	}
}

func TestSetupService_GetStatus_MultipleSources(t *testing.T) {
	userStore, sourceStore, _, svc := newTestSetupService()

	// Users exist
	user := &domain.User{
		ID:        "user-1",
		Email:     "admin@example.com",
		TeamID:    "team-123",
		Role:      domain.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	// Multiple sources exist
	for i := 1; i <= 3; i++ {
		source := &domain.Source{
			ID:           domain.GenerateID(),
			Name:         "Source " + string(rune('0'+i)),
			ProviderType: domain.ProviderTypeGitHub,
			Enabled:      true,
			CreatedAt:    time.Now(),
		}
		_ = sourceStore.Save(context.Background(), source)
	}

	status, err := svc.GetStatus(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.SetupComplete {
		t.Error("expected setup to be complete with users and multiple sources")
	}

	if !status.HasSources {
		t.Error("expected HasSources to be true")
	}
}
