package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// CreateSourceRequest represents a request to create a new source
type CreateSourceRequest struct {
	Name         string              `json:"name"`
	ProviderType domain.ProviderType `json:"provider_type"`
	Config       domain.SourceConfig `json:"config"`
	ConnectionID string              `json:"connection_id,omitempty"`
	Containers   []domain.Container  `json:"containers,omitempty"`
}

// UpdateSourceRequest represents a request to update a source
type UpdateSourceRequest struct {
	Name    *string              `json:"name,omitempty"`
	Config  *domain.SourceConfig `json:"config,omitempty"`
	Enabled *bool                `json:"enabled,omitempty"`
}

// SourceService manages data sources (admin operations)
type SourceService interface {
	// Create creates a new source (admin only)
	Create(ctx context.Context, creatorID string, req CreateSourceRequest) (*domain.Source, error)

	// Get retrieves a source by ID
	Get(ctx context.Context, id string) (*domain.Source, error)

	// List retrieves all sources
	List(ctx context.Context) ([]*domain.Source, error)

	// ListWithSummary retrieves all sources with document counts
	ListWithSummary(ctx context.Context) ([]*domain.SourceSummary, error)

	// Update updates a source (admin only)
	Update(ctx context.Context, id string, req UpdateSourceRequest) (*domain.Source, error)

	// Delete deletes a source and all its documents (admin only)
	Delete(ctx context.Context, id string) error

	// Enable enables a source
	Enable(ctx context.Context, id string) error

	// Disable disables a source
	Disable(ctx context.Context, id string) error

	// UpdateContainers updates the containers for a source
	// Empty slice means index all containers
	UpdateContainers(ctx context.Context, id string, containers []domain.Container) error
}
