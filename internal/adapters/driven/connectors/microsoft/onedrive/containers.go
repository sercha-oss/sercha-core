package onedrive

import (
	"context"
	"fmt"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors/microsoft"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure ContainerLister implements the interface.
var _ driven.ContainerLister = (*ContainerLister)(nil)

// ContainerLister lists OneDrive folders accessible with the given credentials.
type ContainerLister struct {
	client *microsoft.Client
}

// NewContainerLister creates a ContainerLister with the given token provider.
func NewContainerLister(tokenProvider driven.TokenProvider, config *Config) *ContainerLister {
	if config == nil {
		config = DefaultConfig()
	}

	clientConfig := &microsoft.ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   config.RateLimitRPS,
		RequestTimeout: config.RequestTimeout,
		MaxRetries:     config.MaxRetries,
	}

	return &ContainerLister{
		client: microsoft.NewClient(tokenProvider, clientConfig),
	}
}

// ListContainers lists folders in the user's OneDrive.
// If parentID is empty, lists root-level folders. Otherwise lists children of the specified folder.
// Only returns folders (not files).
func (l *ContainerLister) ListContainers(ctx context.Context, cursor string, parentID string) ([]*driven.Container, string, error) {
	var resp *microsoft.DriveItemsResponse
	var err error

	if cursor != "" {
		// Use nextLink for pagination
		err = l.client.GetNextPage(ctx, cursor, &resp)
	} else {
		// Determine which folder to list
		folderID := "root"
		if parentID != "" {
			// Parse parentID format "id:name" - extract the ID part
			parts := strings.SplitN(parentID, ":", 2)
			folderID = parts[0]
		}
		resp, err = l.client.GetDriveItems(ctx, folderID)
	}

	if err != nil {
		return nil, "", fmt.Errorf("list drive items: %w", err)
	}

	containers := make([]*driven.Container, 0)
	for _, item := range resp.Value {
		// Only include folders
		if !item.IsFolder() {
			continue
		}

		// Store ID:Name format so connector can match paths recursively
		containerID := fmt.Sprintf("%s:%s", item.ID, item.Name)
		container := &driven.Container{
			ID:   containerID,
			Name: item.Name,
			Type: "folder",
			Metadata: map[string]string{
				"web_url":           item.WebURL,
				"created_datetime":  item.CreatedDateTime.Format("2006-01-02"),
				"modified_datetime": item.LastModifiedDateTime.Format("2006-01-02"),
			},
		}

		if item.Folder != nil {
			container.Description = fmt.Sprintf("Folder with %d items", item.Folder.ChildCount)
			container.Metadata["child_count"] = fmt.Sprintf("%d", item.Folder.ChildCount)
			// Enable folder navigation if there are child items
			container.HasChildren = item.Folder.ChildCount > 0
		}

		containers = append(containers, container)
	}

	nextCursor := ""
	if resp.NextLink != "" {
		nextCursor = resp.NextLink
	}

	return containers, nextCursor, nil
}

// ContainerListerFactory creates ContainerListers for OneDrive connections.
type ContainerListerFactory struct {
	connectionStore driven.ConnectionStore
	tokenFactory    driven.TokenProviderFactory
}

// NewContainerListerFactory creates a factory for OneDrive container listers.
func NewContainerListerFactory(
	connectionStore driven.ConnectionStore,
	tokenFactory driven.TokenProviderFactory,
) *ContainerListerFactory {
	return &ContainerListerFactory{
		connectionStore: connectionStore,
		tokenFactory:    tokenFactory,
	}
}

// Create creates a ContainerLister for a OneDrive connection.
func (f *ContainerListerFactory) Create(ctx context.Context, connectionID string) (driven.ContainerLister, error) {
	tokenProvider, err := f.tokenFactory.Create(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("create token provider: %w", err)
	}

	return NewContainerLister(tokenProvider, nil), nil
}
