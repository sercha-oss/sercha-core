package notion

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Builder implements the interface.
var _ driven.ConnectorBuilder = (*Builder)(nil)

// Builder creates Notion connectors.
type Builder struct {
	config *Config
}

// NewBuilder creates a new Notion connector builder.
func NewBuilder() *Builder {
	return &Builder{
		config: DefaultConfig(),
	}
}

// NewBuilderWithConfig creates a builder with custom configuration.
func NewBuilderWithConfig(config *Config) *Builder {
	return &Builder{
		config: config,
	}
}

// Type returns the provider type.
func (b *Builder) Type() domain.ProviderType {
	return domain.ProviderTypeNotion
}

// Build creates a Notion connector scoped to a specific page or database.
// containerID format: Notion page/database UUID (e.g., "a1b2c3d4-e5f6-...")
// If containerID is empty, the connector indexes all accessible content.
func (b *Builder) Build(ctx context.Context, tokenProvider driven.TokenProvider, containerID string) (driven.Connector, error) {
	return NewConnector(tokenProvider, containerID, b.config), nil
}

// SupportsOAuth returns true - Notion supports OAuth2.
func (b *Builder) SupportsOAuth() bool {
	return true
}

// OAuthConfig returns OAuth configuration for Notion.
func (b *Builder) OAuthConfig() *driven.OAuthConfig {
	return &driven.OAuthConfig{
		AuthURL:     "https://api.notion.com/v1/oauth/authorize",
		TokenURL:    "https://api.notion.com/v1/oauth/token",
		Scopes:      []string{}, // Notion doesn't use traditional scopes
		UserInfoURL: "https://api.notion.com/v1/users/me",
	}
}

// SupportsContainerSelection returns true - Notion supports page/database selection.
func (b *Builder) SupportsContainerSelection() bool {
	return true
}
