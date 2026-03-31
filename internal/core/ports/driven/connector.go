package driven

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// Connector fetches documents from a source provider.
// Connectors are created by ConnectorBuilder with resolved credentials.
type Connector interface {
	// Type returns the provider type.
	Type() domain.ProviderType

	// ValidateConfig validates source configuration.
	ValidateConfig(config domain.SourceConfig) error

	// FetchChanges fetches document changes since last sync.
	// Returns changes, next cursor, and error.
	// The cursor enables incremental sync - pass empty string for full sync.
	FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error)

	// FetchDocument fetches a single document by external ID.
	// Returns the document, content hash, and error.
	FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error)

	// TestConnection tests the connection to the source.
	TestConnection(ctx context.Context, source *domain.Source) error
}

// ConnectorBuilder creates connector instances for a specific provider type.
// Each provider has its own builder registered with the ConnectorFactory.
type ConnectorBuilder interface {
	// Type returns the provider type this builder creates.
	Type() domain.ProviderType

	// Build creates a connector scoped to a specific container.
	// The containerID comes from source.SelectedContainers (one per sync job).
	// For providers that don't support container selection, containerID may be empty.
	// The TokenProvider handles credential retrieval and OAuth token refresh.
	Build(ctx context.Context, tokenProvider TokenProvider, containerID string) (Connector, error)

	// SupportsOAuth returns true if this connector uses OAuth authentication.
	SupportsOAuth() bool

	// OAuthConfig returns OAuth configuration for this provider.
	// Returns nil if the provider doesn't support OAuth.
	OAuthConfig() *OAuthConfig

	// SupportsContainerSelection returns true if this connector supports container picking.
	// If true, admins can select specific containers (repos, drives, spaces) to index.
	// If false, the connector indexes all accessible content.
	SupportsContainerSelection() bool
}

// OAuthConfig contains OAuth settings for a provider.
type OAuthConfig struct {
	// AuthURL is the authorization endpoint
	AuthURL string

	// TokenURL is the token exchange endpoint
	TokenURL string

	// Scopes are the required OAuth scopes
	Scopes []string

	// UserInfoURL is the endpoint to fetch user info (optional)
	UserInfoURL string
}

// ConnectorFactory manages connector builders and creates connectors.
type ConnectorFactory interface {
	// Register registers a connector builder for a provider type.
	Register(builder ConnectorBuilder)

	// Create creates a connector for the given source, scoped to a container.
	// Called by SyncOrchestrator once per container in source.SelectedContainers.
	// For providers without container selection, containerID may be empty.
	Create(ctx context.Context, source *domain.Source, containerID string) (Connector, error)

	// SupportedTypes returns all registered provider types.
	SupportedTypes() []domain.ProviderType

	// GetBuilder returns the builder for a provider type.
	GetBuilder(providerType domain.ProviderType) (ConnectorBuilder, error)

	// SupportsOAuth returns true if the provider supports OAuth.
	SupportsOAuth(providerType domain.ProviderType) bool

	// GetOAuthConfig returns OAuth config for a provider.
	GetOAuthConfig(providerType domain.ProviderType) *OAuthConfig
}

// OAuthHandler handles OAuth flow for a provider.
type OAuthHandler interface {
	// DefaultConfig returns the default OAuth configuration (auth URL, token URL, scopes)
	DefaultConfig() OAuthConfig

	// BuildAuthURL constructs the OAuth authorization URL with PKCE
	BuildAuthURL(clientID, redirectURI, state, codeChallenge string, scopes []string) string

	// ExchangeCode exchanges an authorization code for access tokens
	ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*OAuthToken, error)

	// GetUserInfo retrieves user information using an access token
	GetUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error)

	// RefreshToken refreshes an expired access token
	RefreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error)
}

// OAuthHandlerFactory creates OAuth handlers for specific providers.
// This abstracts the connectors.Factory dependency.
type OAuthHandlerFactory interface {
	// GetOAuthHandler returns an OAuth handler for the given provider.
	// Returns nil if the provider doesn't support OAuth or isn't implemented.
	GetOAuthHandler(providerType domain.ProviderType) OAuthHandler
}

// OAuthToken represents OAuth tokens from a provider.
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int    // Seconds until expiry
	TokenType    string // Usually "Bearer"
	Scope        string // Space-separated scopes
}

// OAuthUserInfo represents user info from OAuth provider.
type OAuthUserInfo struct {
	ID       string // Provider-specific user ID
	Email    string
	Name     string
	ImageURL string
}

// ContentExtractor extracts text content from various file formats
type ContentExtractor interface {
	// Extract extracts text content from raw data
	Extract(ctx context.Context, data []byte, mimeType string) (string, error)

	// SupportedTypes returns supported MIME types
	SupportedTypes() []string
}

// Chunker splits document content into searchable chunks
type Chunker interface {
	// Chunk splits content into chunks
	Chunk(content string, opts ChunkOptions) []ChunkResult
}

// ChunkOptions configures chunking behavior
type ChunkOptions struct {
	MaxChunkSize int // Maximum characters per chunk
	Overlap      int // Character overlap between chunks
}

// ChunkResult represents a chunk with position info
type ChunkResult struct {
	Content   string
	StartChar int
	EndChar   int
	Position  int
}

// DefaultChunkOptions returns sensible defaults
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		MaxChunkSize: 1000,
		Overlap:      200,
	}
}
