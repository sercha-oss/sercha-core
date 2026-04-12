package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// DocumentResult represents a document-level search result from BM25.
type DocumentResult struct {
	DocumentID string
	SourceID   string
	Title      string
	Content    string  // Full document content (for snippet extraction)
	Score      float64
}

// VectorSearchResult represents a chunk-level vector search result with content.
type VectorSearchResult struct {
	ChunkID    string
	DocumentID string
	Content    string
	Distance   float64
}

// SearchEngine handles search indexing and querying
type SearchEngine interface {
	// Index indexes chunks for a document (legacy, kept for backward compat)
	Index(ctx context.Context, chunks []*domain.Chunk) error

	// IndexDocument indexes a full document for BM25 text search.
	// Uses document_id as the OpenSearch document ID (upsert semantics).
	IndexDocument(ctx context.Context, doc *domain.DocumentContent) error

	// Search performs a search query (legacy chunk-level search)
	Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error)

	// SearchDocuments performs a BM25 text search returning document-level results.
	SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]DocumentResult, int, error)

	// Delete deletes chunks by IDs
	Delete(ctx context.Context, chunkIDs []string) error

	// DeleteByDocument deletes all indexed data for a document
	DeleteByDocument(ctx context.Context, documentID string) error

	// DeleteByDocuments deletes all indexed data for multiple documents in a single operation
	DeleteByDocuments(ctx context.Context, documentIDs []string) error

	// DeleteBySource deletes all indexed data for a source
	DeleteBySource(ctx context.Context, sourceID string) error

	// HealthCheck verifies the search engine is available
	HealthCheck(ctx context.Context) error

	// Count returns the total number of indexed documents
	Count(ctx context.Context) (int64, error)

	// GetDocument retrieves a document's full indexed content by document ID.
	// Returns domain.ErrNotFound if the document is not in the search index.
	GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error)
}

// VectorIndex handles vector similarity search using a standalone embeddings table.
// This interface allows for dedicated vector store implementations (e.g., pgvector).
type VectorIndex interface {
	// Index adds a single embedding to the index
	Index(ctx context.Context, id string, documentID string, embedding []float32) error

	// IndexBatch adds multiple embeddings with their chunk content
	IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error

	// Search finds similar vectors, returns chunk IDs and distances
	Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error)

	// SearchWithContent finds similar vectors and returns chunk content alongside IDs/distances.
	// sourceIDs optionally filters results to specific sources (nil or empty = no filter).
	SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string) ([]VectorSearchResult, error)

	// Delete removes a single embedding by chunk ID
	Delete(ctx context.Context, id string) error

	// DeleteBatch removes multiple embeddings by chunk IDs
	DeleteBatch(ctx context.Context, ids []string) error

	// DeleteByDocument removes all embeddings for a document
	DeleteByDocument(ctx context.Context, documentID string) error

	// DeleteByDocuments removes all embeddings for multiple documents in a single operation
	DeleteByDocuments(ctx context.Context, documentIDs []string) error

	// HealthCheck verifies the vector store is available
	HealthCheck(ctx context.Context) error
}
