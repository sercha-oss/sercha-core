package pgvector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	driven "github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure interface compliance at compile time
var _ driven.VectorIndex = (*VectorIndex)(nil)

// EnsureTable creates the embeddings table if it doesn't exist.
// Called during startup to ensure the table is ready.
func (v *VectorIndex) EnsureTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS embeddings (
			chunk_id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			source_id TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			embedding vector(%d) NOT NULL
		)
	`, v.dimensions)

	if _, err := v.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create embeddings table: %w", err)
	}

	// Migrations: add columns if table existed before these changes
	_, _ = v.pool.Exec(ctx, `ALTER TABLE embeddings ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT ''`)
	_, _ = v.pool.Exec(ctx, `ALTER TABLE embeddings ADD COLUMN IF NOT EXISTS source_id TEXT NOT NULL DEFAULT ''`)

	// Create index on document_id for deletion queries
	_, err := v.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_embeddings_document_id ON embeddings(document_id)`)
	if err != nil {
		return fmt.Errorf("failed to create document_id index: %w", err)
	}

	// Create index on source_id for source filtering
	_, err = v.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_embeddings_source_id ON embeddings(source_id)`)
	if err != nil {
		return fmt.Errorf("failed to create source_id index: %w", err)
	}

	// Create HNSW vector index for similarity search
	idxQuery := `
		CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings
		USING hnsw (embedding vector_cosine_ops)
	`
	if _, err := v.pool.Exec(ctx, idxQuery); err != nil {
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	return nil
}

// Index inserts or updates a single embedding
func (v *VectorIndex) Index(ctx context.Context, id string, documentID string, embedding []float32) error {
	if len(embedding) != v.dimensions {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", v.dimensions, len(embedding))
	}

	vec := pgvector.NewVector(embedding)

	query := `
		INSERT INTO embeddings (chunk_id, document_id, embedding)
		VALUES ($1, $2, $3)
		ON CONFLICT (chunk_id) DO UPDATE SET embedding = EXCLUDED.embedding
	`
	_, err := v.pool.Exec(ctx, query, id, documentID, vec)
	if err != nil {
		return fmt.Errorf("failed to index embedding: %w", err)
	}

	return nil
}

// SearchWithContent finds similar vectors and returns chunk content alongside IDs/distances.
// sourceIDs optionally filters results to specific sources (nil or empty = no filter).
// documentFilter applies the three-case document-id contract; see domain.DocumentIDFilter godoc.
func (v *VectorIndex) SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string, documentFilter *domain.DocumentIDFilter) ([]driven.VectorSearchResult, error) {
	// Three-case contract on documentFilter:
	//   - nil or !Apply: no document_id predicate.
	//   - Apply && len(IDs) == 0: authoritative deny-all; short-circuit to empty results.
	//   - Apply && len(IDs) > 0: allow-list bound into the WHERE clause.
	if documentFilter != nil && documentFilter.Apply && len(documentFilter.IDs) == 0 {
		return []driven.VectorSearchResult{}, nil
	}

	if len(embedding) != v.dimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", v.dimensions, len(embedding))
	}

	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}

	vec := pgvector.NewVector(embedding)

	var rows pgx.Rows
	var err error

	// Build WHERE clause conditions
	hasSourceFilter := len(sourceIDs) > 0
	hasDocFilter := documentFilter.IsAllowList()

	if hasSourceFilter && hasDocFilter {
		query := fmt.Sprintf(`
			SELECT chunk_id, document_id, content, embedding %s $1::vector AS distance
			FROM embeddings
			WHERE source_id = ANY($3) AND document_id = ANY($4)
			ORDER BY distance
			LIMIT $2
		`, v.distOp)
		rows, err = v.pool.Query(ctx, query, vec, k, sourceIDs, documentFilter.IDs)
	} else if hasSourceFilter {
		query := fmt.Sprintf(`
			SELECT chunk_id, document_id, content, embedding %s $1::vector AS distance
			FROM embeddings
			WHERE source_id = ANY($3)
			ORDER BY distance
			LIMIT $2
		`, v.distOp)
		rows, err = v.pool.Query(ctx, query, vec, k, sourceIDs)
	} else if hasDocFilter {
		query := fmt.Sprintf(`
			SELECT chunk_id, document_id, content, embedding %s $1::vector AS distance
			FROM embeddings
			WHERE document_id = ANY($3)
			ORDER BY distance
			LIMIT $2
		`, v.distOp)
		rows, err = v.pool.Query(ctx, query, vec, k, documentFilter.IDs)
	} else {
		query := fmt.Sprintf(`
			SELECT chunk_id, document_id, content, embedding %s $1::vector AS distance
			FROM embeddings
			ORDER BY distance
			LIMIT $2
		`, v.distOp)
		rows, err = v.pool.Query(ctx, query, vec, k)
	}
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []driven.VectorSearchResult
	for rows.Next() {
		var r driven.VectorSearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Content, &r.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// IndexBatch inserts or updates multiple embeddings with their chunk content using batch operations
func (v *VectorIndex) IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error {
	if len(ids) != len(embeddings) {
		return fmt.Errorf("ids and embeddings count mismatch: %d vs %d", len(ids), len(embeddings))
	}
	if len(ids) != len(documentIDs) {
		return fmt.Errorf("ids and documentIDs count mismatch: %d vs %d", len(ids), len(documentIDs))
	}
	if len(ids) != len(sourceIDs) {
		return fmt.Errorf("ids and sourceIDs count mismatch: %d vs %d", len(ids), len(sourceIDs))
	}
	if len(ids) != len(contents) {
		return fmt.Errorf("ids and contents count mismatch: %d vs %d", len(ids), len(contents))
	}

	if len(ids) == 0 {
		return nil
	}

	// Validate dimensions
	for i, emb := range embeddings {
		if len(emb) != v.dimensions {
			return fmt.Errorf("embedding %d dimension mismatch: expected %d, got %d", i, v.dimensions, len(emb))
		}
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO embeddings (chunk_id, document_id, source_id, content, embedding)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chunk_id) DO UPDATE SET source_id = EXCLUDED.source_id, content = EXCLUDED.content, embedding = EXCLUDED.embedding
	`

	for i, id := range ids {
		vec := pgvector.NewVector(embeddings[i])
		batch.Queue(query, id, documentIDs[i], sourceIDs[i], contents[i], vec)
	}

	br := v.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	var errs []error
	for i := 0; i < len(ids); i++ {
		if _, err := br.Exec(); err != nil {
			errs = append(errs, fmt.Errorf("batch item %d (%s): %w", i, ids[i], err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("batch indexing had %d errors, first: %w", len(errs), errs[0])
	}

	return nil
}

// Search finds the k most similar chunks by embedding distance
func (v *VectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	if len(embedding) != v.dimensions {
		return nil, nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", v.dimensions, len(embedding))
	}

	if k <= 0 {
		return nil, nil, fmt.Errorf("k must be positive, got %d", k)
	}

	vec := pgvector.NewVector(embedding)

	query := fmt.Sprintf(`
		SELECT chunk_id, embedding %s $1::vector AS distance
		FROM embeddings
		ORDER BY distance
		LIMIT $2
	`, v.distOp)

	rows, err := v.pool.Query(ctx, query, vec, k)
	if err != nil {
		return nil, nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var ids []string
	var distances []float64

	for rows.Next() {
		var id string
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}
		ids = append(ids, id)
		distances = append(distances, distance)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return ids, distances, nil
}

// Delete removes a single embedding by chunk ID
func (v *VectorIndex) Delete(ctx context.Context, id string) error {
	_, err := v.pool.Exec(ctx, `DELETE FROM embeddings WHERE chunk_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}
	return nil
}

// DeleteBatch removes multiple embeddings by chunk IDs
func (v *VectorIndex) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := v.pool.Exec(ctx, `DELETE FROM embeddings WHERE chunk_id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}
	return nil
}

// DeleteByDocument removes all embeddings for a document
func (v *VectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	_, err := v.pool.Exec(ctx, `DELETE FROM embeddings WHERE document_id = $1`, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings by document: %w", err)
	}
	return nil
}

// DeleteByDocuments removes all embeddings for multiple documents in a single operation
func (v *VectorIndex) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	_, err := v.pool.Exec(ctx, `DELETE FROM embeddings WHERE document_id = ANY($1)`, documentIDs)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings by documents: %w", err)
	}
	return nil
}

// DeleteBySourceAndContainer removes all embeddings for a specific container within a source
// Joins with documents table to filter by container_id from document metadata
func (v *VectorIndex) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	query := `
		DELETE FROM embeddings
		WHERE document_id IN (
			SELECT id FROM documents
			WHERE source_id = $1 AND metadata->>'container_id' = $2
		)
	`
	_, err := v.pool.Exec(ctx, query, sourceID, containerID)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings by source and container: %w", err)
	}
	return nil
}

// HealthCheck verifies the connection and vector extension availability
func (v *VectorIndex) HealthCheck(ctx context.Context) error {
	if err := v.pool.Ping(ctx); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

	var extName string
	err := v.pool.QueryRow(ctx, `SELECT extname FROM pg_extension WHERE extname = 'vector'`).Scan(&extName)
	if err != nil {
		return fmt.Errorf("vector extension not available: %w", err)
	}

	return nil
}
