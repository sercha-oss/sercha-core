package driven

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// VespaDeployer handles Vespa schema deployment
type VespaDeployer interface {
	// Deploy deploys the Vespa application package.
	// If embeddingDim is nil, deploys BM25-only schema.
	// If embeddingDim is provided, deploys hybrid schema with that dimension.
	// If existingPkg is provided, merges our schema into it instead of using embedded services.xml.
	Deploy(ctx context.Context, endpoint string, embeddingDim *int, existingPkg *AppPackage) (*domain.VespaDeployResult, error)

	// FetchAppPackage retrieves the currently deployed application package from Vespa.
	// Returns nil if no application is deployed.
	FetchAppPackage(ctx context.Context, endpoint string) (*AppPackage, error)

	// GetSchemaInfo retrieves information about the currently deployed schema
	GetSchemaInfo(ctx context.Context, endpoint string) (*SchemaInfo, error)

	// HealthCheck verifies the Vespa cluster is healthy
	HealthCheck(ctx context.Context, endpoint string) error

	// GetMetrics fetches cluster metrics from the Vespa metrics API.
	// metricsEndpoint should be the metrics proxy endpoint (typically port 19092).
	GetMetrics(ctx context.Context, metricsEndpoint string) (*domain.VespaMetrics, error)
}

// AppPackage represents a Vespa application package
type AppPackage struct {
	// Raw file contents
	ServicesXML string            `json:"services_xml"`
	HostsXML    string            `json:"hosts_xml,omitempty"`
	Schemas     map[string]string `json:"schemas"` // filename -> content

	// Parsed cluster info
	ClusterInfo *domain.VespaClusterInfo `json:"cluster_info,omitempty"`
}

// SchemaInfo contains information about the deployed Vespa schema
type SchemaInfo struct {
	// Deployed indicates if any schema is deployed
	Deployed bool

	// SchemaMode indicates the type of schema (bm25 or hybrid)
	SchemaMode domain.VespaSchemaMode

	// EmbeddingDim is the embedding dimension if hybrid
	EmbeddingDim int

	// Version is the schema version string
	Version string
}

// VespaConfigStore persists Vespa configuration
type VespaConfigStore interface {
	// GetVespaConfig retrieves Vespa config for a team
	GetVespaConfig(ctx context.Context, teamID string) (*domain.VespaConfig, error)

	// SaveVespaConfig persists Vespa config
	SaveVespaConfig(ctx context.Context, config *domain.VespaConfig) error
}
