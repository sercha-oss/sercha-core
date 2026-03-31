package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// ProviderService provides information about available data source providers.
// Credentials are managed via environment variables, not this service.
type ProviderService interface {
	// List returns all available providers with their configuration status.
	// Configuration status is based on environment variables.
	List(ctx context.Context) ([]*ProviderListItem, error)
}

// ProviderListItem represents a provider in the list response.
type ProviderListItem struct {
	Type         domain.ProviderType   `json:"type"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	AuthMethods  []domain.AuthMethod   `json:"auth_methods"`
	Configured   bool                  `json:"configured"`
	Enabled      bool                  `json:"enabled"`
	DocsURL      string                `json:"docs_url,omitempty"`
}

