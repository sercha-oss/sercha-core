package services

import (
	"context"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Ensure installationService implements InstallationService
var _ driving.InstallationService = (*installationService)(nil)

// InstallationServiceConfig holds configuration for the installation service.
type InstallationServiceConfig struct {
	// InstallationStore manages installation persistence.
	InstallationStore driven.InstallationStore

	// SourceStore manages source persistence (for checking usage).
	SourceStore driven.SourceStore

	// ContainerListerFactory creates container listers for providers.
	ContainerListerFactory driven.ContainerListerFactory

	// TokenProviderFactory creates token providers for testing connections.
	TokenProviderFactory driven.TokenProviderFactory
}

// installationService implements the InstallationService interface.
type installationService struct {
	installationStore      driven.InstallationStore
	sourceStore            driven.SourceStore
	containerListerFactory driven.ContainerListerFactory
	tokenProviderFactory   driven.TokenProviderFactory
}

// NewInstallationService creates a new installation service.
func NewInstallationService(cfg InstallationServiceConfig) driving.InstallationService {
	return &installationService{
		installationStore:      cfg.InstallationStore,
		sourceStore:            cfg.SourceStore,
		containerListerFactory: cfg.ContainerListerFactory,
		tokenProviderFactory:   cfg.TokenProviderFactory,
	}
}

// Create creates a new installation for non-OAuth connectors.
func (s *installationService) Create(ctx context.Context, req driving.CreateInstallationRequest) (*domain.InstallationSummary, error) {
	// Validate input
	if req.Name == "" {
		return nil, domain.ErrInvalidInput
	}
	if req.ProviderType == "" {
		return nil, domain.ErrInvalidInput
	}
	if req.APIKey == "" {
		return nil, domain.ErrInvalidInput
	}

	now := time.Now()
	inst := &domain.Installation{
		ID:           generateID(),
		Name:         req.Name,
		ProviderType: req.ProviderType,
		AuthMethod:   domain.AuthMethodAPIKey,
		Secrets: &domain.InstallationSecrets{
			APIKey: req.APIKey,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.installationStore.Save(ctx, inst); err != nil {
		return nil, err
	}

	return inst.ToSummary(), nil
}

// List returns all installations (summaries without secrets).
func (s *installationService) List(ctx context.Context) ([]*domain.InstallationSummary, error) {
	return s.installationStore.List(ctx)
}

// Get retrieves an installation by ID (summary without secrets).
func (s *installationService) Get(ctx context.Context, id string) (*domain.InstallationSummary, error) {
	inst, err := s.installationStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return inst.ToSummary(), nil
}

// Delete removes an installation.
func (s *installationService) Delete(ctx context.Context, id string) error {
	// Check if installation is in use by any sources
	count, err := s.sourceStore.CountByInstallation(ctx, id)
	if err != nil {
		return fmt.Errorf("check installation usage: %w", err)
	}
	if count > 0 {
		return domain.ErrInUse
	}

	return s.installationStore.Delete(ctx, id)
}

// ListByProvider returns installations for a specific provider type.
func (s *installationService) ListByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.InstallationSummary, error) {
	return s.installationStore.GetByProvider(ctx, providerType)
}

// ListContainers lists available containers for an installation.
func (s *installationService) ListContainers(ctx context.Context, installationID string, cursor string) (*driving.ListContainersResponse, error) {
	// Get the installation to determine provider type
	inst, err := s.installationStore.Get(ctx, installationID)
	if err != nil {
		return nil, err
	}

	// Check if container listing is supported
	if s.containerListerFactory == nil {
		return nil, fmt.Errorf("container listing not available")
	}

	if !s.containerListerFactory.SupportsContainerSelection(inst.ProviderType) {
		// Provider doesn't support container selection - return empty list
		return &driving.ListContainersResponse{
			Containers: []*driven.Container{},
			NextCursor: "",
			HasMore:    false,
		}, nil
	}

	// Create container lister for this installation
	lister, err := s.containerListerFactory.Create(ctx, inst.ProviderType, installationID)
	if err != nil {
		return nil, fmt.Errorf("create container lister: %w", err)
	}

	// List containers
	containers, nextCursor, err := lister.ListContainers(ctx, cursor)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	return &driving.ListContainersResponse{
		Containers: containers,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// TestConnection tests if the installation's credentials are still valid.
func (s *installationService) TestConnection(ctx context.Context, id string) error {
	// Get the installation with secrets
	inst, err := s.installationStore.Get(ctx, id)
	if err != nil {
		return err
	}

	// Create a token provider to test the credentials
	if s.tokenProviderFactory == nil {
		return fmt.Errorf("token provider factory not available")
	}

	tokenProvider, err := s.tokenProviderFactory.CreateFromInstallation(ctx, inst)
	if err != nil {
		return fmt.Errorf("create token provider: %w", err)
	}

	// Try to get a token - this will refresh if needed
	_, err = tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("credentials invalid: %w", err)
	}

	// Update last used timestamp
	_ = s.installationStore.UpdateLastUsed(ctx, id)

	return nil
}
