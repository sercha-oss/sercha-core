package services

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Ensure providerService implements ProviderService
var _ driving.ProviderService = (*providerService)(nil)

// providerService implements the ProviderService interface.
// It provides information about available providers based on environment configuration.
type providerService struct {
	configProvider driven.ConfigProvider
}

// NewProviderService creates a new ProviderService.
func NewProviderService(configProvider driven.ConfigProvider) driving.ProviderService {
	return &providerService{
		configProvider: configProvider,
	}
}

// List returns all available providers with their configuration status.
func (s *providerService) List(ctx context.Context) ([]*driving.ProviderListItem, error) {
	// Build list with all core providers
	coreProviders := domain.CoreProviders()
	items := make([]*driving.ProviderListItem, 0, len(coreProviders))

	for _, providerType := range coreProviders {
		info := providerMetadata(providerType)
		item := &driving.ProviderListItem{
			Type:        providerType,
			Name:        info.name,
			Description: info.description,
			AuthMethods: info.authMethods,
			DocsURL:     info.docsURL,
			Configured:  s.configProvider.IsOAuthConfigured(providerType),
			Enabled:     s.configProvider.IsOAuthConfigured(providerType),
		}

		items = append(items, item)
	}

	return items, nil
}

// providerMeta holds static metadata about a provider.
type providerMeta struct {
	name        string
	description string
	authMethods []domain.AuthMethod
	docsURL     string
}

// providerMetadata returns static metadata for a provider type.
func providerMetadata(pt domain.ProviderType) providerMeta {
	switch pt {
	case domain.ProviderTypeGitHub:
		return providerMeta{
			name:        "GitHub",
			description: "Index repositories, issues, pull requests, and wikis",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodPAT},
			docsURL:     "https://docs.sercha.dev/connectors/github",
		}
	case domain.ProviderTypeGitLab:
		return providerMeta{
			name:        "GitLab",
			description: "Index repositories, issues, merge requests, and wikis",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodPAT},
			docsURL:     "https://docs.sercha.dev/connectors/gitlab",
		}
	case domain.ProviderTypeSlack:
		return providerMeta{
			name:        "Slack",
			description: "Index channels, threads, and messages",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
			docsURL:     "https://docs.sercha.dev/connectors/slack",
		}
	case domain.ProviderTypeNotion:
		return providerMeta{
			name:        "Notion",
			description: "Index pages, databases, and documents",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
			docsURL:     "https://docs.sercha.dev/connectors/notion",
		}
	case domain.ProviderTypeConfluence:
		return providerMeta{
			name:        "Confluence",
			description: "Index spaces, pages, and blog posts",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodAPIKey},
			docsURL:     "https://docs.sercha.dev/connectors/confluence",
		}
	case domain.ProviderTypeJira:
		return providerMeta{
			name:        "Jira",
			description: "Index projects, issues, and comments",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodAPIKey},
			docsURL:     "https://docs.sercha.dev/connectors/jira",
		}
	case domain.ProviderTypeGoogleDrive:
		return providerMeta{
			name:        "Google Drive",
			description: "Index files, folders, and shared drives",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodServiceAccount},
			docsURL:     "https://docs.sercha.dev/connectors/google-drive",
		}
	case domain.ProviderTypeGoogleDocs:
		return providerMeta{
			name:        "Google Docs",
			description: "Index Google Docs documents",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodServiceAccount},
			docsURL:     "https://docs.sercha.dev/connectors/google-docs",
		}
	case domain.ProviderTypeLinear:
		return providerMeta{
			name:        "Linear",
			description: "Index issues, projects, and comments",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2, domain.AuthMethodAPIKey},
			docsURL:     "https://docs.sercha.dev/connectors/linear",
		}
	case domain.ProviderTypeDropbox:
		return providerMeta{
			name:        "Dropbox",
			description: "Index files and folders",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
			docsURL:     "https://docs.sercha.dev/connectors/dropbox",
		}
	case domain.ProviderTypeS3:
		return providerMeta{
			name:        "Amazon S3",
			description: "Index objects from S3 buckets",
			authMethods: []domain.AuthMethod{domain.AuthMethodAPIKey},
			docsURL:     "https://docs.sercha.dev/connectors/s3",
		}
	default:
		return providerMeta{
			name:        string(pt),
			description: "Data source connector",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
		}
	}
}
