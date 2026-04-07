package pgvector

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds pgvector connection and index configuration
type Config struct {
	// URL is the PostgreSQL connection string with pgvector extension
	URL string

	// Dimensions is the embedding vector dimension (default: 1536 for OpenAI ada-002)
	Dimensions int

	// IndexType is the vector index type: "hnsw" or "ivfflat" (default: "hnsw")
	IndexType string

	// DistanceMetric is the distance function: "cosine", "l2", or "inner_product" (default: "cosine")
	DistanceMetric string

	// MaxOpenConns is the maximum number of open connections in the pool
	MaxOpenConns int32

	// MinConns is the minimum number of connections in the pool
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum idle time of a connection
	MaxConnIdleTime time.Duration

	// ConnTimeout is the connection timeout
	ConnTimeout time.Duration
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		URL:             "",
		Dimensions:      1536,
		IndexType:       "hnsw",
		DistanceMetric:  "cosine",
		MaxOpenConns:    10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		ConnTimeout:     10 * time.Second,
	}
}

// VectorIndex implements the driven.VectorIndex interface using pgvector
type VectorIndex struct {
	pool       *pgxpool.Pool
	dimensions int
	distOp     string // distance operator: <=>, <->, or <#>
}

// New creates a new pgvector VectorIndex adapter
func New(ctx context.Context, cfg Config) (*VectorIndex, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("pgvector URL is required")
	}

	// Parse the connection config
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgvector URL: %w", err)
	}

	// Apply pool settings
	poolConfig.MaxConns = cfg.MaxOpenConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnTimeout

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgvector connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping pgvector: %w", err)
	}

	// Determine distance operator based on metric
	var distOp string
	switch cfg.DistanceMetric {
	case "l2":
		distOp = "<->" // Euclidean distance
	case "inner_product":
		distOp = "<#>" // negative inner product
	case "cosine", "":
		distOp = "<=>" // cosine distance
	default:
		pool.Close()
		return nil, fmt.Errorf("unknown distance metric: %s (use: cosine, l2, inner_product)", cfg.DistanceMetric)
	}

	return &VectorIndex{
		pool:       pool,
		dimensions: cfg.Dimensions,
		distOp:     distOp,
	}, nil
}

// Close closes the connection pool
func (v *VectorIndex) Close() {
	if v.pool != nil {
		v.pool.Close()
	}
}
