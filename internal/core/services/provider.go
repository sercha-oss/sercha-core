package services

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
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
	// Build list with only implemented providers
	implementedProviders := []domain.ProviderType{
		domain.ProviderTypeGitHub,
		domain.ProviderTypeNotion,
		domain.ProviderTypeLocalFS,
		domain.ProviderTypeOneDrive,
	}
	items := make([]*driving.ProviderListItem, 0, len(implementedProviders))

	for _, providerType := range implementedProviders {
		info := providerMetadata(providerType)
		platform := domain.PlatformFor(providerType)
		item := &driving.ProviderListItem{
			Type:        providerType,
			Name:        info.name,
			Description: info.description,
			AuthMethods: info.authMethods,
			DocsURL:     info.docsURL,
			Configured:  s.configProvider.IsOAuthConfigured(platform),
			Enabled:     s.configProvider.IsOAuthConfigured(platform),
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
	case domain.ProviderTypeNotion:
		return providerMeta{
			name:        "Notion",
			description: "Index pages, databases, and workspace content",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
			docsURL:     "https://docs.sercha.dev/connectors/notion",
		}
	case domain.ProviderTypeLocalFS:
		return providerMeta{
			name:        "Local Filesystem",
			description: "Index files and directories from the local filesystem",
			authMethods: []domain.AuthMethod{},
			docsURL:     "https://docs.sercha.dev/connectors/localfs",
		}
	case domain.ProviderTypeOneDrive:
		return providerMeta{
			name:        "Microsoft OneDrive",
			description: "Index files and folders from Microsoft OneDrive",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
			docsURL:     "https://docs.sercha.dev/connectors/onedrive",
		}
	default:
		return providerMeta{
			name:        string(pt),
			description: "Data source connector",
			authMethods: []domain.AuthMethod{domain.AuthMethodOAuth2},
		}
	}
}
