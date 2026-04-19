package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure connectionService implements ConnectionService
var _ driving.ConnectionService = (*connectionService)(nil)

// ConnectionServiceConfig holds configuration for the connection service.
type ConnectionServiceConfig struct {
	// ConnectionStore manages connection persistence.
	ConnectionStore driven.ConnectionStore

	// SourceStore manages source persistence (for checking usage).
	SourceStore driven.SourceStore

	// ContainerListerFactory creates container listers for providers.
	ContainerListerFactory driven.ContainerListerFactory

	// TokenProviderFactory creates token providers for testing connections.
	TokenProviderFactory driven.TokenProviderFactory
}

// connectionService implements the ConnectionService interface.
type connectionService struct {
	connectionStore        driven.ConnectionStore
	sourceStore            driven.SourceStore
	containerListerFactory driven.ContainerListerFactory
	tokenProviderFactory   driven.TokenProviderFactory
}

// NewConnectionService creates a new connection service.
func NewConnectionService(cfg ConnectionServiceConfig) driving.ConnectionService {
	return &connectionService{
		connectionStore:        cfg.ConnectionStore,
		sourceStore:            cfg.SourceStore,
		containerListerFactory: cfg.ContainerListerFactory,
		tokenProviderFactory:   cfg.TokenProviderFactory,
	}
}

// Create creates a new connection for non-OAuth connectors.
func (s *connectionService) Create(ctx context.Context, req driving.CreateConnectionRequest) (*domain.ConnectionSummary, error) {
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
	conn := &domain.Connection{
		ID:           generateID(),
		Name:         req.Name,
		ProviderType: req.ProviderType,
		AuthMethod:   domain.AuthMethodAPIKey,
		Secrets: &domain.ConnectionSecrets{
			APIKey: req.APIKey,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.connectionStore.Save(ctx, conn); err != nil {
		return nil, err
	}

	return conn.ToSummary(), nil
}

// List returns all connections (summaries without secrets).
func (s *connectionService) List(ctx context.Context) ([]*domain.ConnectionSummary, error) {
	return s.connectionStore.List(ctx)
}

// Get retrieves a connection by ID (summary without secrets).
func (s *connectionService) Get(ctx context.Context, id string) (*domain.ConnectionSummary, error) {
	conn, err := s.connectionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return conn.ToSummary(), nil
}

// Delete removes a connection.
func (s *connectionService) Delete(ctx context.Context, id string) error {
	// Check if connection is in use by any sources
	count, err := s.sourceStore.CountByConnection(ctx, id)
	if err != nil {
		return fmt.Errorf("check connection usage: %w", err)
	}
	if count > 0 {
		return domain.ErrInUse
	}

	return s.connectionStore.Delete(ctx, id)
}

// ListByProvider returns connections for a specific provider type.
func (s *connectionService) ListByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.ConnectionSummary, error) {
	// Map provider type to platform for store lookup
	platform := domain.PlatformFor(providerType)
	return s.connectionStore.GetByPlatform(ctx, platform)
}

// ListContainers lists available containers for a connection.
// parentID is optional - if provided, lists children of that container (for folder navigation).
func (s *connectionService) ListContainers(ctx context.Context, connectionID string, cursor string, parentID string) (*driving.ListContainersResponse, error) {
	// Get the connection to determine provider type
	conn, err := s.connectionStore.Get(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	// Check if container listing is supported
	if s.containerListerFactory == nil {
		return nil, fmt.Errorf("container listing not available")
	}

	if !s.containerListerFactory.SupportsContainerSelection(conn.ProviderType) {
		// Provider doesn't support container selection - return empty list
		return &driving.ListContainersResponse{
			Containers: []*driven.Container{},
			NextCursor: "",
			HasMore:    false,
		}, nil
	}

	// Create container lister for this connection
	lister, err := s.containerListerFactory.Create(ctx, conn.ProviderType, connectionID)
	if err != nil {
		return nil, fmt.Errorf("create container lister: %w", err)
	}

	// List containers
	containers, nextCursor, err := lister.ListContainers(ctx, cursor, parentID)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	return &driving.ListContainersResponse{
		Containers: containers,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// TestConnection tests if the connection's credentials are still valid.
func (s *connectionService) TestConnection(ctx context.Context, id string) error {
	// Get the connection with secrets
	conn, err := s.connectionStore.Get(ctx, id)
	if err != nil {
		return err
	}

	// Create a token provider to test the credentials
	if s.tokenProviderFactory == nil {
		return fmt.Errorf("token provider factory not available")
	}

	tokenProvider, err := s.tokenProviderFactory.CreateFromConnection(ctx, conn)
	if err != nil {
		return fmt.Errorf("create token provider: %w", err)
	}

	// Try to get a token - this will refresh if needed
	_, err = tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("credentials invalid: %w", err)
	}

	// Update last used timestamp
	_ = s.connectionStore.UpdateLastUsed(ctx, id)

	return nil
}
