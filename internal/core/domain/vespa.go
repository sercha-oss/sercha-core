package domain

import "time"

// VespaSchemaMode represents the current schema deployment mode
type VespaSchemaMode string

const (
	// VespacSchemaModeNone indicates no schema is deployed
	VespacSchemaModeNone VespaSchemaMode = ""

	// VespacSchemaModeBM25 indicates BM25-only schema (no embeddings)
	VespacSchemaModeBM25 VespaSchemaMode = "bm25"

	// VespacSchemaModeHybrid indicates hybrid schema (BM25 + embeddings)
	VespacSchemaModeHybrid VespaSchemaMode = "hybrid"
)

// VespaConfig holds Vespa connection and schema state for a team
type VespaConfig struct {
	TeamID string `json:"team_id"`

	// Connection settings
	Endpoint  string `json:"endpoint"`
	Connected bool   `json:"connected"`
	DevMode   bool   `json:"dev_mode"` // true = we deployed services.xml, false = we only added schema

	// Schema state
	SchemaMode        VespaSchemaMode `json:"schema_mode"`
	EmbeddingDim      int             `json:"embedding_dim,omitempty"`
	EmbeddingProvider AIProvider      `json:"embedding_provider,omitempty"`
	SchemaVersion     string          `json:"schema_version"`

	// Cluster info (populated in production mode)
	ClusterInfo *VespaClusterInfo `json:"cluster_info,omitempty"`

	// Timestamps
	ConnectedAt time.Time `json:"connected_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsConnected returns true if Vespa is connected and schema deployed
func (c *VespaConfig) IsConnected() bool {
	return c.Connected && c.SchemaMode != VespacSchemaModeNone
}

// HasEmbeddings returns true if the schema supports embeddings
func (c *VespaConfig) HasEmbeddings() bool {
	return c.SchemaMode == VespacSchemaModeHybrid && c.EmbeddingDim > 0
}

// CanUpgradeToHybrid returns true if schema can be upgraded from BM25 to hybrid
func (c *VespaConfig) CanUpgradeToHybrid() bool {
	return c.SchemaMode == VespacSchemaModeBM25
}

// DefaultVespaConfig returns a default unconfigured Vespa config
func DefaultVespaConfig(teamID string) *VespaConfig {
	return &VespaConfig{
		TeamID:    teamID,
		Endpoint:  "http://vespa:19071",
		Connected: false,
		UpdatedAt: time.Now(),
	}
}

// VespaDeployResult represents the result of a schema deployment
type VespaDeployResult struct {
	Success       bool            `json:"success"`
	SchemaMode    VespaSchemaMode `json:"schema_mode"`
	EmbeddingDim  int             `json:"embedding_dim,omitempty"`
	SchemaVersion string          `json:"schema_version"`
	Upgraded      bool            `json:"upgraded"`
	Message       string          `json:"message,omitempty"`
}

// VespaClusterInfo represents parsed information about a Vespa cluster
type VespaClusterInfo struct {
	// Raw XML content (for persistence and future reference)
	ServicesXML string `json:"services_xml,omitempty"`
	HostsXML    string `json:"hosts_xml,omitempty"`

	// Parsed cluster information
	ContentClusters   []VespaContentCluster   `json:"content_clusters,omitempty"`
	ContainerClusters []VespaContainerCluster `json:"container_clusters,omitempty"`
	Hosts             []VespaHost             `json:"hosts,omitempty"`
	Schemas           []string                `json:"schemas,omitempty"`

	// Our schema status
	OurSchemaDeployed bool `json:"our_schema_deployed"`
}

// VespaContentCluster represents a Vespa content cluster
type VespaContentCluster struct {
	ID         string   `json:"id"`
	Redundancy int      `json:"redundancy,omitempty"`
	Nodes      []string `json:"nodes,omitempty"`
	Documents  []string `json:"documents,omitempty"`
}

// VespaContainerCluster represents a Vespa container cluster
type VespaContainerCluster struct {
	ID       string   `json:"id"`
	Port     int      `json:"port,omitempty"`
	Nodes    []string `json:"nodes,omitempty"`
	HasFeed  bool     `json:"has_feed"`
	HasQuery bool     `json:"has_query"`
}

// VespaHost represents a host in the Vespa cluster
type VespaHost struct {
	Alias    string `json:"alias"`
	Hostname string `json:"hostname"`
}

// VespaMetrics represents aggregated metrics from the Vespa cluster
type VespaMetrics struct {
	// Document metrics
	Documents VespaDocumentMetrics `json:"documents"`

	// Storage metrics
	Storage VespaStorageMetrics `json:"storage"`

	// Query performance metrics
	QueryPerformance VespaQueryMetrics `json:"query_performance"`

	// Feed metrics
	Feed VespaFeedMetrics `json:"feed"`

	// Per-service metrics
	Services []VespaServiceMetrics `json:"services"`

	// Timestamp of metrics collection
	Timestamp int64 `json:"timestamp"`
}

// VespaDocumentMetrics contains document count metrics
type VespaDocumentMetrics struct {
	Total   int64 `json:"total"`
	Ready   int64 `json:"ready"`
	Active  int64 `json:"active"`
	Removed int64 `json:"removed"`
}

// VespaStorageMetrics contains storage utilization metrics
type VespaStorageMetrics struct {
	// Host disk metrics (filesystem where Vespa stores data)
	DiskUsedBytes     int64   `json:"disk_used_bytes"`
	DiskUsedPercent   float64 `json:"disk_used_percent"`
	// Actual Vespa data size (documentdb + transaction log)
	DataSizeBytes int64 `json:"data_size_bytes"`
	// Memory metrics
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
}

// VespaQueryMetrics contains query performance metrics
type VespaQueryMetrics struct {
	TotalQueries     int64   `json:"total_queries"`
	QueriesPerSecond float64 `json:"queries_per_second"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	MaxLatencyMs     float64 `json:"max_latency_ms"`
	HitsPerQuery     float64 `json:"hits_per_query"`
}

// VespaFeedMetrics contains document feed metrics
type VespaFeedMetrics struct {
	PutOperations    int64   `json:"put_operations"`
	UpdateOperations int64   `json:"update_operations"`
	RemoveOperations int64   `json:"remove_operations"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// VespaServiceMetrics contains metrics for a single Vespa service
type VespaServiceMetrics struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	MemoryMB  int64   `json:"memory_mb"`
	CPUUtil   float64 `json:"cpu_util"`
	Timestamp int64   `json:"timestamp"`
}
