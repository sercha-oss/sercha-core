package github

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure ContainerLister implements the interface.
var _ driven.ContainerLister = (*ContainerLister)(nil)

// ContainerLister lists GitHub repositories accessible with an installation's credentials.
type ContainerLister struct {
	client *Client
}

// NewContainerLister creates a ContainerLister with the given token provider.
func NewContainerLister(tokenProvider driven.TokenProvider, baseURL string) *ContainerLister {
	return &ContainerLister{
		client: NewClient(tokenProvider, baseURL),
	}
}

// ListContainers lists all repositories accessible to the authenticated user.
// Returns repositories as containers in the format "owner/repo".
func (l *ContainerLister) ListContainers(ctx context.Context, cursor string, _ string) ([]*driven.Container, string, error) {
	// parentID ignored - GitHub repos are flat
	resp, err := l.client.ListAccessibleRepos(ctx, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("list repos: %w", err)
	}

	containers := make([]*driven.Container, len(resp.Repos))
	for i, repo := range resp.Repos {
		containers[i] = &driven.Container{
			ID:          repo.FullName, // "owner/repo" format
			Name:        repo.Name,
			Description: repo.Description,
			Type:        "repository",
			Metadata: map[string]string{
				"owner":          repo.Owner,
				"private":        fmt.Sprintf("%t", repo.Private),
				"archived":       fmt.Sprintf("%t", repo.Archived),
				"default_branch": repo.DefaultBranch,
				"html_url":       repo.HTMLURL,
			},
		}
	}

	return containers, resp.NextCursor, nil
}

// ContainerListerFactory creates ContainerListers for GitHub installations.
type ContainerListerFactory struct {
	installationStore driven.ConnectionStore
	tokenFactory      driven.TokenProviderFactory
	baseURL           string
}

// NewContainerListerFactory creates a factory for GitHub container listers.
func NewContainerListerFactory(
	installationStore driven.ConnectionStore,
	tokenFactory driven.TokenProviderFactory,
	baseURL string,
) *ContainerListerFactory {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &ContainerListerFactory{
		installationStore: installationStore,
		tokenFactory:      tokenFactory,
		baseURL:           baseURL,
	}
}

// Create creates a ContainerLister for a GitHub installation.
func (f *ContainerListerFactory) Create(ctx context.Context, installationID string) (driven.ContainerLister, error) {
	tokenProvider, err := f.tokenFactory.Create(ctx, installationID)
	if err != nil {
		return nil, fmt.Errorf("create token provider: %w", err)
	}

	return NewContainerLister(tokenProvider, f.baseURL), nil
}
