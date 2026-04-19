package onedrive

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Builder implements the interface.
var _ driven.ConnectorBuilder = (*Builder)(nil)

// Builder creates OneDrive connectors.
type Builder struct {
	config *Config
}

// NewBuilder creates a new OneDrive connector builder.
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
	return domain.ProviderTypeOneDrive
}

// Build creates a OneDrive connector scoped to a specific folder.
// containerID format: OneDrive folder ID (e.g., "01BYE5RZ6QN3ZWBTUFOFD3GSPGOHDJD36K")
// If containerID is empty, the connector indexes all accessible content.
func (b *Builder) Build(ctx context.Context, tokenProvider driven.TokenProvider, containerID string) (driven.Connector, error) {
	return NewConnector(tokenProvider, containerID, b.config), nil
}

// SupportsOAuth returns true - OneDrive supports OAuth2.
func (b *Builder) SupportsOAuth() bool {
	return true
}

// OAuthConfig returns OAuth configuration for OneDrive (Microsoft platform).
func (b *Builder) OAuthConfig() *driven.OAuthConfig {
	return &driven.OAuthConfig{
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:      []string{"Files.Read", "User.Read", "offline_access"},
		UserInfoURL: "https://graph.microsoft.com/v1.0/me",
	}
}

// SupportsContainerSelection returns true - OneDrive supports folder selection.
func (b *Builder) SupportsContainerSelection() bool {
	return true
}
