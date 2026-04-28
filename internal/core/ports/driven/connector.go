package driven

import (
	"context"
	"errors"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// ErrInventoryNotSupported is returned from Inventory by connectors whose
// upstream API natively signals deletions (so the orchestrator's
// snapshot-diff reconciliation is not needed and would be wasteful). OneDrive
// is the canonical example — its delta API emits @removed tombstones, so it
// declares no ReconciliationScopes and this sentinel guards against future
// callers invoking Inventory directly.
var ErrInventoryNotSupported = errors.New("inventory not supported by this connector")

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

	// ReconciliationScopes reports external-ID prefixes for which this
	// connector can produce a complete current-state Inventory. The
	// orchestrator uses the declared scopes to detect deletions: anything
	// stored under a scope's prefix that is absent from the latest Inventory
	// is treated as deleted upstream.
	//
	// Declare a scope only when the upstream API does NOT natively signal
	// deletes (GitHub issues/PRs, Notion pages and database entries).
	// Connectors whose delta API emits tombstones (OneDrive) should return
	// nil — reconciliation would be redundant and wasteful.
	ReconciliationScopes() []string

	// Inventory returns the complete set of canonical IDs currently present
	// upstream within the given scope prefix. The prefix is one of the
	// values from ReconciliationScopes.
	//
	// Contract:
	//   - The returned slice MUST be complete — every canonical ID the
	//     upstream source of truth holds under this prefix, right now.
	//   - On any partial-result condition (paginated fetch failure, rate
	//     limit exhausted mid-walk, etc.) Inventory MUST return an error
	//     rather than a partial slice. A partial inventory causes false
	//     positives in the orchestrator's delete diff and permanently
	//     erases real documents.
	//   - Archived / trashed items are treated as "no longer present" and
	//     MUST be omitted from the inventory (not included).
	//   - Connectors that do not support inventory (empty scopes) return
	//     ErrInventoryNotSupported.
	Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error)

	// RESTClient returns a RESTClient that reuses this connector's auth,
	// rate-limiting, and retry plumbing for callers that need to invoke
	// upstream REST endpoints not covered by the typed methods above.
	//
	// MUST always return a non-nil value. Connectors without a REST surface
	// (e.g. local filesystem) return a sentinel whose Do always returns
	// ErrRESTUnsupported — callers type-check the error rather than the
	// return value.
	RESTClient() RESTClient
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

// OAuthHandlerFactory creates OAuth handlers for specific platforms.
// This abstracts the connectors.Factory dependency.
type OAuthHandlerFactory interface {
	// GetOAuthHandler returns an OAuth handler for the given platform.
	// Returns nil if the platform doesn't support OAuth or isn't implemented.
	GetOAuthHandler(platform domain.PlatformType) OAuthHandler
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
