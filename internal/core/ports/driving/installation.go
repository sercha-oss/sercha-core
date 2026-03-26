package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// CreateInstallationRequest represents a request to create an installation for non-OAuth connectors.
// @Description Request to create an installation for API key or path-based connectors
type CreateInstallationRequest struct {
	// Name is a human-readable name for the installation.
	Name string `json:"name" example:"Local Test Docs"`

	// ProviderType is the data source provider.
	ProviderType domain.ProviderType `json:"provider_type" example:"localfs"`

	// APIKey is the authentication credential (or path for localfs).
	APIKey string `json:"api_key" example:"/data/test-docs"`
}

// InstallationService manages connector installations (OAuth connections, API keys, etc.).
// Installations represent authenticated connections to external data sources.
type InstallationService interface {
	// Create creates a new installation for non-OAuth connectors (API key, path-based).
	// Returns ErrInvalidInput if required fields are missing.
	Create(ctx context.Context, req CreateInstallationRequest) (*domain.InstallationSummary, error)

	// List returns all installations (summaries without secrets).
	List(ctx context.Context) ([]*domain.InstallationSummary, error)

	// Get retrieves an installation by ID (summary without secrets).
	Get(ctx context.Context, id string) (*domain.InstallationSummary, error)

	// Delete removes an installation.
	// Returns ErrNotFound if installation doesn't exist.
	// Returns ErrInUse if installation is referenced by sources.
	Delete(ctx context.Context, id string) error

	// ListByProvider returns installations for a specific provider type.
	ListByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.InstallationSummary, error)

	// ListContainers lists available containers (repos, drives, spaces) for an installation.
	// Supports pagination via cursor.
	ListContainers(ctx context.Context, installationID string, cursor string) (*ListContainersResponse, error)

	// TestConnection tests if the installation's credentials are still valid.
	TestConnection(ctx context.Context, id string) error
}

// ListContainersResponse contains the paginated list of containers.
// @Description Paginated list of containers available for indexing
type ListContainersResponse struct {
	// Containers is the list of available containers (repos, drives, spaces, etc.)
	Containers []*driven.Container `json:"containers"`

	// NextCursor is the cursor for the next page, empty if no more pages.
	NextCursor string `json:"next_cursor,omitempty"`

	// HasMore indicates if there are more containers to fetch.
	HasMore bool `json:"has_more"`
}

// InstallationSummaryResponse is the API response for an installation.
// @Description Installation summary (secrets not exposed)
type InstallationSummaryResponse struct {
	// ID is the unique installation identifier.
	ID string `json:"id" example:"inst_abc123def456"`

	// Name is a human-readable name for the installation.
	Name string `json:"name" example:"GitHub (octocat)"`

	// ProviderType is the data source provider.
	ProviderType domain.ProviderType `json:"provider_type" example:"github"`

	// AuthMethod is the authentication method used.
	AuthMethod domain.AuthMethod `json:"auth_method" example:"oauth2"`

	// AccountID is the external account identifier (email, username).
	AccountID string `json:"account_id,omitempty" example:"octocat"`

	// OAuthExpiry is when OAuth tokens expire (if applicable).
	OAuthExpiry *string `json:"oauth_expiry,omitempty" example:"2024-01-15T12:00:00Z"`

	// CreatedAt is when the installation was created.
	CreatedAt string `json:"created_at" example:"2024-01-15T10:00:00Z"`

	// LastUsedAt is when the installation was last used for syncing.
	LastUsedAt *string `json:"last_used_at,omitempty" example:"2024-01-15T11:00:00Z"`

	// SourceCount is the number of sources using this installation.
	SourceCount int `json:"source_count" example:"3"`
}

// ContainerResponse represents a container in API responses.
// @Description A container (repository, drive, space) that can be selected for indexing
type ContainerResponse struct {
	// ID is the provider-specific container identifier.
	ID string `json:"id" example:"octocat/hello-world"`

	// Name is the display name.
	Name string `json:"name" example:"hello-world"`

	// Description is an optional description.
	Description string `json:"description,omitempty" example:"My first repository"`

	// Type identifies the container type.
	Type string `json:"type" example:"repository"`

	// Metadata contains provider-specific additional data.
	Metadata map[string]string `json:"metadata,omitempty"`
}
