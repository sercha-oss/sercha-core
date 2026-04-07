package pgvector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure interface compliance at compile time
var _ driven.VectorIndex = (*VectorIndex)(nil)

// Index adds or updates a single embedding for a chunk
func (v *VectorIndex) Index(ctx context.Context, id string, embedding []float32) error {
	if len(embedding) != v.dimensions {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", v.dimensions, len(embedding))
	}

	// Convert to pgvector type
	vec := pgvector.NewVector(embedding)

	// Update the embedding column in the chunks table
	query := `UPDATE chunks SET embedding = $1 WHERE id = $2`
	result, err := v.pool.Exec(ctx, query, vec, id)
	if err != nil {
		return fmt.Errorf("failed to index embedding: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("chunk not found: %s", id)
	}

	return nil
}

// IndexBatch adds or updates multiple embeddings using batch operations
func (v *VectorIndex) IndexBatch(ctx context.Context, ids []string, embeddings [][]float32) error {
	if len(ids) != len(embeddings) {
		return fmt.Errorf("ids and embeddings count mismatch: %d vs %d", len(ids), len(embeddings))
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

	// Use pgx batch for efficient bulk operations
	batch := &pgx.Batch{}
	query := `UPDATE chunks SET embedding = $1 WHERE id = $2`

	for i, id := range ids {
		vec := pgvector.NewVector(embeddings[i])
		batch.Queue(query, vec, id)
	}

	// Execute batch
	br := v.pool.SendBatch(ctx, batch)
	defer br.Close()

	// Check results
	var errs []error
	for i := 0; i < len(ids); i++ {
		result, err := br.Exec()
		if err != nil {
			errs = append(errs, fmt.Errorf("batch item %d (%s): %w", i, ids[i], err))
			continue
		}
		if result.RowsAffected() == 0 {
			errs = append(errs, fmt.Errorf("batch item %d: chunk not found: %s", i, ids[i]))
		}
	}

	if len(errs) > 0 {
		// Return first error with count of total errors
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

	// Convert to pgvector type
	vec := pgvector.NewVector(embedding)

	// Build query with parameterized distance operator
	// Note: We use string formatting for the operator since it can't be parameterized
	// The operator is validated in New() so this is safe
	query := fmt.Sprintf(`
		SELECT id, embedding %s $1::vector AS distance
		FROM chunks
		WHERE embedding IS NOT NULL
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

// Delete removes the embedding for a single chunk
func (v *VectorIndex) Delete(ctx context.Context, id string) error {
	query := `UPDATE chunks SET embedding = NULL WHERE id = $1`
	_, err := v.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}
	return nil
}

// DeleteBatch removes embeddings for multiple chunks
func (v *VectorIndex) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := `UPDATE chunks SET embedding = NULL WHERE id = ANY($1)`
	_, err := v.pool.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}
	return nil
}

// HealthCheck verifies the connection and vector extension availability
func (v *VectorIndex) HealthCheck(ctx context.Context) error {
	// Test connection
	if err := v.pool.Ping(ctx); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

	// Verify vector extension is available
	var extName string
	err := v.pool.QueryRow(ctx, `SELECT extname FROM pg_extension WHERE extname = 'vector'`).Scan(&extName)
	if err != nil {
		return fmt.Errorf("vector extension not available: %w", err)
	}

	return nil
}
