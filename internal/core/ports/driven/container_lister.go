package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// Container represents a top-level container that can be selected for indexing.
// Examples:
//   - GitHub: repository (owner/repo)
//   - Google Drive: shared drive
//   - Confluence: space
//   - Jira: project
//   - Notion: workspace
//   - Slack: channel
type Container struct {
	// ID is the provider-specific identifier.
	// Format varies by provider:
	//   - GitHub: "owner/repo"
	//   - Google Drive: drive ID
	//   - Confluence: space key
	ID string `json:"id"`

	// Name is the human-readable display name.
	Name string `json:"name"`

	// Description is an optional description of the container.
	Description string `json:"description,omitempty"`

	// Type identifies the container type.
	// Examples: "repository", "shared_drive", "space", "project", "channel"
	Type string `json:"type"`

	// HasChildren indicates if this container has child containers (for folder navigation).
	HasChildren bool `json:"has_children,omitempty"`

	// Metadata contains provider-specific additional data.
	// Examples:
	//   - GitHub: {"owner": "...", "private": "true", "archived": "false"}
	//   - Google Drive: {"shared": "true"}
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ContainerLister lists available containers for a connector.
// Not all connectors support container selection - some index all accessible content.
//
// The ContainerLister is typically retrieved from the ConnectorFactory
// for a specific provider type and installation.
type ContainerLister interface {
	// ListContainers lists containers accessible with the current credentials.
	// Pagination is handled via cursor - pass empty string for the first page.
	// parentID can be used to list children of a specific container (for folder navigation).
	// Pass empty string for root level containers.
	// Returns:
	//   - containers: list of available containers
	//   - nextCursor: cursor for the next page, empty if no more pages
	//   - err: any error that occurred
	ListContainers(ctx context.Context, cursor string, parentID string) ([]*Container, string, error)
}

// ContainerListerFactory creates ContainerListers for different providers.
type ContainerListerFactory interface {
	// Create creates a ContainerLister for the given provider and installation.
	// Returns nil if the provider doesn't support container selection.
	Create(ctx context.Context, providerType domain.ProviderType, installationID string) (ContainerLister, error)

	// SupportsContainerSelection returns true if the provider supports container selection.
	// If false, the connector indexes all accessible content.
	SupportsContainerSelection(providerType domain.ProviderType) bool
}
