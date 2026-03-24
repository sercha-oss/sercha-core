package localfs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure Builder implements the interface.
var _ driven.ConnectorBuilder = (*Builder)(nil)

// Builder creates LocalFS connectors.
type Builder struct {
	config       *Config
	allowedRoots []string // Security: restrict to allowed base paths
}

// NewBuilder creates a new LocalFS connector builder.
// allowedRoots restricts which directories can be indexed (empty = no restriction).
func NewBuilder(allowedRoots []string) *Builder {
	return &Builder{
		config:       DefaultConfig(),
		allowedRoots: allowedRoots,
	}
}

// NewBuilderWithConfig creates a builder with custom configuration.
func NewBuilderWithConfig(config *Config, allowedRoots []string) *Builder {
	return &Builder{
		config:       config,
		allowedRoots: allowedRoots,
	}
}

// Type returns the provider type.
func (b *Builder) Type() domain.ProviderType {
	return domain.ProviderTypeLocalFS
}

// Build creates a LocalFS connector scoped to a specific subdirectory.
// containerID is a relative path from the installation's base path.
// If containerID is empty, the entire base path is indexed.
func (b *Builder) Build(ctx context.Context, tokenProvider driven.TokenProvider, containerID string) (driven.Connector, error) {
	// Get base path from token provider (stored as APIKey in installation)
	basePath, err := tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get base path: %w", err)
	}

	if basePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	// Security: validate base path is allowed
	if !b.isAllowedPath(basePath) {
		return nil, fmt.Errorf("base path not in allowed roots: %s", basePath)
	}

	// Resolve full path
	fullPath := basePath
	if containerID != "" {
		fullPath = filepath.Join(basePath, containerID)
	}

	// Security: ensure resolved path is still under base
	fullPath, err = filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve base path: %w", err)
	}

	if !strings.HasPrefix(fullPath, absBase) {
		return nil, fmt.Errorf("path traversal detected: %s", containerID)
	}

	return NewConnector(fullPath, containerID, b.config), nil
}

// SupportsOAuth returns false - LocalFS uses path-based auth.
func (b *Builder) SupportsOAuth() bool {
	return false
}

// OAuthConfig returns nil - no OAuth support.
func (b *Builder) OAuthConfig() *driven.OAuthConfig {
	return nil
}

// SupportsContainerSelection returns true - containers are subdirectories.
func (b *Builder) SupportsContainerSelection() bool {
	return true
}

// isAllowedPath checks if path is under allowed roots.
func (b *Builder) isAllowedPath(path string) bool {
	if len(b.allowedRoots) == 0 {
		return true // No restrictions
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, root := range b.allowedRoots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absRoot) {
			return true
		}
	}

	return false
}
