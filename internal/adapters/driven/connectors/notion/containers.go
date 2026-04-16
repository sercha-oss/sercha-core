package notion

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure ContainerLister implements the interface.
var _ driven.ContainerLister = (*ContainerLister)(nil)

// ContainerLister lists Notion pages and databases accessible with the given credentials.
type ContainerLister struct {
	client *Client
}

// NewContainerLister creates a ContainerLister with the given token provider.
func NewContainerLister(tokenProvider driven.TokenProvider, config *Config) *ContainerLister {
	if config == nil {
		config = DefaultConfig()
	}
	return &ContainerLister{
		client: NewClient(tokenProvider, config),
	}
}

// ListContainers lists databases and top-level pages accessible to the integration.
// Only returns databases and standalone pages (pages NOT in a database).
// Pages that belong to databases are excluded - they are indexed when their
// parent database is synced, avoiding duplicate indexing and slow sync times.
func (l *ContainerLister) ListContainers(ctx context.Context, cursor string) ([]*driven.Container, string, error) {
	// Search for all pages and databases
	resp, err := l.client.Search(ctx, nil, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("search: %w", err)
	}

	containers := make([]*driven.Container, 0, len(resp.Results))
	for _, result := range resp.Results {
		if result.Archived {
			// Skip archived items
			continue
		}

		// Skip pages that belong to a database - they will be indexed
		// when their parent database is synced
		if result.Object == "page" && result.Parent.Type == "database_id" {
			continue
		}

		container := &driven.Container{
			ID:   result.ID,
			Type: result.Object, // "page" or "database"
			Metadata: map[string]string{
				"url":               result.URL,
				"created_time":      result.CreatedTime.Format("2006-01-02"),
				"last_edited_time":  result.LastEditedTime.Format("2006-01-02"),
			},
		}

		switch result.Object {
		case "page":
			container.Name = GetPageTitle(result.Properties)
			if container.Name == "" {
				container.Name = "Untitled Page"
			}
			container.Metadata["parent_type"] = result.Parent.Type

		case "database":
			container.Name = ExtractPlainText(result.Title)
			if container.Name == "" {
				container.Name = "Untitled Database"
			}
			container.Description = fmt.Sprintf("Database with %d properties", len(result.Properties))
			container.Metadata["parent_type"] = result.Parent.Type
		}

		containers = append(containers, container)
	}

	nextCursor := ""
	if resp.HasMore {
		nextCursor = resp.NextCursor
	}

	return containers, nextCursor, nil
}

// ContainerListerFactory creates ContainerListers for Notion connections.
type ContainerListerFactory struct {
	connectionStore driven.ConnectionStore
	tokenFactory    driven.TokenProviderFactory
}

// NewContainerListerFactory creates a factory for Notion container listers.
func NewContainerListerFactory(
	connectionStore driven.ConnectionStore,
	tokenFactory driven.TokenProviderFactory,
) *ContainerListerFactory {
	return &ContainerListerFactory{
		connectionStore: connectionStore,
		tokenFactory:    tokenFactory,
	}
}

// Create creates a ContainerLister for a Notion connection.
func (f *ContainerListerFactory) Create(ctx context.Context, connectionID string) (driven.ContainerLister, error) {
	tokenProvider, err := f.tokenFactory.Create(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("create token provider: %w", err)
	}

	return NewContainerLister(tokenProvider, nil), nil
}
