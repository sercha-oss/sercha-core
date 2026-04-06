package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// VespaAdminService manages Vespa connection and schema deployment
type VespaAdminService interface {
	// Connect connects to Vespa and deploys the schema.
	// If embedding is configured, deploys hybrid schema; otherwise BM25-only.
	// This is an UPSERT operation - can upgrade BM25 to hybrid but never downgrade.
	Connect(ctx context.Context, req ConnectVespaRequest) (*VespaStatus, error)

	// Status returns the current Vespa connection and schema status
	Status(ctx context.Context) (*VespaStatus, error)

	// HealthCheck performs a health check on the Vespa cluster
	HealthCheck(ctx context.Context) error

	// GetMetrics retrieves detailed metrics from the Vespa cluster
	GetMetrics(ctx context.Context) (*domain.VespaMetrics, error)
}

// ConnectVespaRequest represents a request to connect to Vespa
type ConnectVespaRequest struct {
	// Endpoint is the Vespa cluster endpoint (optional, uses stored/default if empty)
	Endpoint string `json:"endpoint,omitempty"`

	// DevMode controls deployment behavior:
	// - true: Deploy our full app package (services.xml + schema) - for local development
	// - false: Fetch existing app package, add our schema, redeploy - for production clusters
	DevMode bool `json:"dev_mode"`
}

// VespaStatus represents the current state of the Vespa connection
type VespaStatus struct {
	// Connected indicates if Vespa is reachable and schema is deployed
	Connected bool `json:"connected"`

	// Endpoint is the Vespa cluster endpoint
	Endpoint string `json:"endpoint"`

	// DevMode indicates if we deployed full app package (true) or just added schema (false)
	DevMode bool `json:"dev_mode"`

	// SchemaMode indicates the deployed schema type (bm25, hybrid)
	SchemaMode domain.VespaSchemaMode `json:"schema_mode"`

	// EmbeddingsEnabled indicates if hybrid search is available
	EmbeddingsEnabled bool `json:"embeddings_enabled"`

	// EmbeddingDim is the embedding dimension if embeddings are enabled
	EmbeddingDim int `json:"embedding_dim,omitempty"`

	// EmbeddingProvider is the provider used for embeddings
	EmbeddingProvider domain.AIProvider `json:"embedding_provider,omitempty"`

	// SchemaVersion is the deployed schema version
	SchemaVersion string `json:"schema_version"`

	// CanUpgrade indicates if schema can be upgraded to hybrid
	CanUpgrade bool `json:"can_upgrade"`

	// ReindexRequired indicates if documents need reindexing
	ReindexRequired bool `json:"reindex_required"`

	// Healthy indicates if the Vespa cluster is healthy
	Healthy bool `json:"healthy"`

	// IndexedChunks is the number of chunks currently indexed in Vespa
	IndexedChunks int64 `json:"indexed_chunks"`

	// ClusterInfo contains parsed information about the Vespa cluster (production mode only)
	ClusterInfo *domain.VespaClusterInfo `json:"cluster_info,omitempty"`
}
