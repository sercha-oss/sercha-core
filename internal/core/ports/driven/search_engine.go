package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// SearchEngine handles search indexing and querying
type SearchEngine interface {
	// Index indexes chunks for a document
	Index(ctx context.Context, chunks []*domain.Chunk) error

	// Search performs a search query
	Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error)

	// Delete deletes chunks by IDs
	Delete(ctx context.Context, chunkIDs []string) error

	// DeleteByDocument deletes all chunks for a document
	DeleteByDocument(ctx context.Context, documentID string) error

	// DeleteBySource deletes all chunks for a source
	DeleteBySource(ctx context.Context, sourceID string) error

	// HealthCheck verifies the search engine is available
	HealthCheck(ctx context.Context) error

	// Count returns the total number of indexed chunks
	Count(ctx context.Context) (int64, error)
}

// VectorIndex handles vector similarity search
// This interface allows for dedicated vector store implementations (e.g., pgvector).
type VectorIndex interface {
	// Index adds vectors to the index
	Index(ctx context.Context, id string, embedding []float32) error

	// IndexBatch adds multiple vectors
	IndexBatch(ctx context.Context, ids []string, embeddings [][]float32) error

	// Search finds similar vectors
	Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error)

	// Delete removes a vector
	Delete(ctx context.Context, id string) error

	// DeleteBatch removes multiple vectors
	DeleteBatch(ctx context.Context, ids []string) error

	// HealthCheck verifies the vector store is available
	HealthCheck(ctx context.Context) error
}
