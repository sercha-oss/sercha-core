package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// CreateConnectionRequest represents a request to create a connection for non-OAuth connectors.
// @Description Request to create a connection for API key or path-based connectors
type CreateConnectionRequest struct {
	// Name is a human-readable name for the connection.
	Name string `json:"name" example:"Local Test Docs"`

	// ProviderType is the data source provider.
	ProviderType domain.ProviderType `json:"provider_type" example:"localfs"`

	// APIKey is the authentication credential (or path for localfs).
	APIKey string `json:"api_key" example:"/data/test-docs"`
}

// ConnectionService manages connector connections (OAuth connections, API keys, etc.).
// Connections represent authenticated links to external data sources.
type ConnectionService interface {
	// Create creates a new connection for non-OAuth connectors (API key, path-based).
	// Returns ErrInvalidInput if required fields are missing.
	Create(ctx context.Context, req CreateConnectionRequest) (*domain.ConnectionSummary, error)

	// List returns all connections (summaries without secrets).
	List(ctx context.Context) ([]*domain.ConnectionSummary, error)

	// Get retrieves a connection by ID (summary without secrets).
	Get(ctx context.Context, id string) (*domain.ConnectionSummary, error)

	// Delete removes a connection.
	// Returns ErrNotFound if connection doesn't exist.
	// Returns ErrInUse if connection is referenced by sources.
	Delete(ctx context.Context, id string) error

	// ListByProvider returns connections for a specific provider type.
	ListByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.ConnectionSummary, error)

	// ListContainers lists available containers (repos, drives, spaces) for a connection.
	// Supports pagination via cursor.
	ListContainers(ctx context.Context, connectionID string, cursor string) (*ListContainersResponse, error)

	// TestConnection tests if the connection's credentials are still valid.
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

// ConnectionSummaryResponse is the API response for a connection.
// @Description Connection summary (secrets not exposed)
type ConnectionSummaryResponse struct {
	// ID is the unique connection identifier.
	ID string `json:"id" example:"conn_abc123def456"`

	// Name is a human-readable name for the connection.
	Name string `json:"name" example:"GitHub (octocat)"`

	// ProviderType is the data source provider.
	ProviderType domain.ProviderType `json:"provider_type" example:"github"`

	// AuthMethod is the authentication method used.
	AuthMethod domain.AuthMethod `json:"auth_method" example:"oauth2"`

	// AccountID is the external account identifier (email, username).
	AccountID string `json:"account_id,omitempty" example:"octocat"`

	// OAuthExpiry is when OAuth tokens expire (if applicable).
	OAuthExpiry *string `json:"oauth_expiry,omitempty" example:"2024-01-15T12:00:00Z"`

	// CreatedAt is when the connection was created.
	CreatedAt string `json:"created_at" example:"2024-01-15T10:00:00Z"`

	// LastUsedAt is when the connection was last used for syncing.
	LastUsedAt *string `json:"last_used_at,omitempty" example:"2024-01-15T11:00:00Z"`

	// SourceCount is the number of sources using this connection.
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
