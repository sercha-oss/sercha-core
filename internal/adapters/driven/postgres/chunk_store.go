package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.ChunkStore = (*ChunkStore)(nil)

// ChunkStore implements driven.ChunkStore using PostgreSQL
// Note: Embeddings are stored in the vector store (pgvector), not here
type ChunkStore struct {
	db *DB
}

// NewChunkStore creates a new ChunkStore
func NewChunkStore(db *DB) *ChunkStore {
	return &ChunkStore{db: db}
}

// Save creates or updates a chunk
func (s *ChunkStore) Save(ctx context.Context, chunk *domain.Chunk) error {
	query := `
		INSERT INTO chunks (id, document_id, source_id, content, position, start_char, end_char, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			position = EXCLUDED.position,
			start_char = EXCLUDED.start_char,
			end_char = EXCLUDED.end_char
	`

	_, err := s.db.ExecContext(ctx, query,
		chunk.ID,
		chunk.DocumentID,
		chunk.SourceID,
		chunk.Content,
		chunk.Position,
		chunk.StartChar,
		chunk.EndChar,
		chunk.CreatedAt,
	)
	return err
}

// SaveBatch saves multiple chunks in a transaction
func (s *ChunkStore) SaveBatch(ctx context.Context, chunks []*domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	return s.db.Transaction(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO chunks (id, document_id, source_id, content, position, start_char, end_char, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				content = EXCLUDED.content,
				position = EXCLUDED.position,
				start_char = EXCLUDED.start_char,
				end_char = EXCLUDED.end_char
		`

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, chunk := range chunks {
			_, err = stmt.ExecContext(ctx,
				chunk.ID,
				chunk.DocumentID,
				chunk.SourceID,
				chunk.Content,
				chunk.Position,
				chunk.StartChar,
				chunk.EndChar,
				chunk.CreatedAt,
			)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// GetByDocument retrieves all chunks for a document
func (s *ChunkStore) GetByDocument(ctx context.Context, documentID string) ([]*domain.Chunk, error) {
	query := `
		SELECT id, document_id, source_id, content, position, start_char, end_char, created_at
		FROM chunks
		WHERE document_id = $1
		ORDER BY position ASC
	`

	rows, err := s.db.QueryContext(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*domain.Chunk
	for rows.Next() {
		var chunk domain.Chunk
		err := rows.Scan(
			&chunk.ID,
			&chunk.DocumentID,
			&chunk.SourceID,
			&chunk.Content,
			&chunk.Position,
			&chunk.StartChar,
			&chunk.EndChar,
			&chunk.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, &chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// Delete deletes a chunk
func (s *ChunkStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM chunks WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// DeleteByDocument deletes all chunks for a document
func (s *ChunkStore) DeleteByDocument(ctx context.Context, documentID string) error {
	query := `DELETE FROM chunks WHERE document_id = $1`
	_, err := s.db.ExecContext(ctx, query, documentID)
	return err
}

// DeleteBySource deletes all chunks for a source
func (s *ChunkStore) DeleteBySource(ctx context.Context, sourceID string) error {
	query := `DELETE FROM chunks WHERE source_id = $1`
	_, err := s.db.ExecContext(ctx, query, sourceID)
	return err
}

// GetChunkIDs returns all chunk IDs for a document (useful for search index cleanup)
func (s *ChunkStore) GetChunkIDs(ctx context.Context, documentID string) ([]string, error) {
	query := `SELECT id FROM chunks WHERE document_id = $1`

	rows, err := s.db.QueryContext(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetChunkIDsBySource returns all chunk IDs for a source (useful for search index cleanup)
func (s *ChunkStore) GetChunkIDsBySource(ctx context.Context, sourceID string) ([]string, error) {
	query := `SELECT id FROM chunks WHERE source_id = $1`

	rows, err := s.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// DeleteBatch deletes multiple chunks by ID
func (s *ChunkStore) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "$" + string(rune('1'+i))
		args[i] = id
	}

	query := `DELETE FROM chunks WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}
