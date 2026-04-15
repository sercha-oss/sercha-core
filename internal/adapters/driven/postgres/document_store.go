package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.DocumentStore = (*DocumentStore)(nil)

// DocumentStore implements driven.DocumentStore using PostgreSQL
type DocumentStore struct {
	db *DB
}

// NewDocumentStore creates a new DocumentStore
func NewDocumentStore(db *DB) *DocumentStore {
	return &DocumentStore{db: db}
}

// Save creates or updates a document
func (s *DocumentStore) Save(ctx context.Context, doc *domain.Document) error {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO documents (id, source_id, external_id, path, title, mime_type, metadata, created_at, updated_at, indexed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			external_id = EXCLUDED.external_id,
			path = EXCLUDED.path,
			title = EXCLUDED.title,
			mime_type = EXCLUDED.mime_type,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at,
			indexed_at = EXCLUDED.indexed_at
	`

	_, err = s.db.ExecContext(ctx, query,
		doc.ID,
		doc.SourceID,
		doc.ExternalID,
		doc.Path,
		doc.Title,
		doc.MimeType,
		metadataJSON,
		doc.CreatedAt,
		doc.UpdatedAt,
		NullTime(&doc.IndexedAt),
	)
	return err
}

// SaveBatch saves multiple documents in a transaction
func (s *DocumentStore) SaveBatch(ctx context.Context, docs []*domain.Document) error {
	if len(docs) == 0 {
		return nil
	}

	return s.db.Transaction(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO documents (id, source_id, external_id, path, title, mime_type, metadata, created_at, updated_at, indexed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO UPDATE SET
				external_id = EXCLUDED.external_id,
				path = EXCLUDED.path,
				title = EXCLUDED.title,
				mime_type = EXCLUDED.mime_type,
				metadata = EXCLUDED.metadata,
				updated_at = EXCLUDED.updated_at,
				indexed_at = EXCLUDED.indexed_at
		`

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()

		for _, doc := range docs {
			metadataJSON, err := json.Marshal(doc.Metadata)
			if err != nil {
				return err
			}

			_, err = stmt.ExecContext(ctx,
				doc.ID,
				doc.SourceID,
				doc.ExternalID,
				doc.Path,
				doc.Title,
				doc.MimeType,
				metadataJSON,
				doc.CreatedAt,
				doc.UpdatedAt,
				NullTime(&doc.IndexedAt),
			)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Get retrieves a document by ID
func (s *DocumentStore) Get(ctx context.Context, id string) (*domain.Document, error) {
	query := `
		SELECT id, source_id, external_id, path, title, mime_type, metadata, created_at, updated_at, indexed_at
		FROM documents
		WHERE id = $1
	`

	return s.scanDocument(s.db.QueryRowContext(ctx, query, id))
}

// GetByExternalID retrieves a document by source and external ID
func (s *DocumentStore) GetByExternalID(ctx context.Context, sourceID, externalID string) (*domain.Document, error) {
	query := `
		SELECT id, source_id, external_id, path, title, mime_type, metadata, created_at, updated_at, indexed_at
		FROM documents
		WHERE source_id = $1 AND external_id = $2
	`

	return s.scanDocument(s.db.QueryRowContext(ctx, query, sourceID, externalID))
}

func (s *DocumentStore) scanDocument(row *sql.Row) (*domain.Document, error) {
	var doc domain.Document
	var metadataJSON []byte
	var path, title, mimeType sql.NullString
	var indexedAt sql.NullTime

	err := row.Scan(
		&doc.ID,
		&doc.SourceID,
		&doc.ExternalID,
		&path,
		&title,
		&mimeType,
		&metadataJSON,
		&doc.CreatedAt,
		&doc.UpdatedAt,
		&indexedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	doc.Path = path.String
	doc.Title = title.String
	doc.MimeType = mimeType.String

	if indexedAt.Valid {
		doc.IndexedAt = indexedAt.Time
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, err
		}
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]string)
	}

	return &doc, nil
}

// GetBySource retrieves all documents for a source with pagination
func (s *DocumentStore) GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error) {
	query := `
		SELECT id, source_id, external_id, path, title, mime_type, metadata, created_at, updated_at, indexed_at
		FROM documents
		WHERE source_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.QueryContext(ctx, query, sourceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return s.scanDocuments(rows)
}

func (s *DocumentStore) scanDocuments(rows *sql.Rows) ([]*domain.Document, error) {
	var docs []*domain.Document
	for rows.Next() {
		var doc domain.Document
		var metadataJSON []byte
		var path, title, mimeType sql.NullString
		var indexedAt sql.NullTime

		err := rows.Scan(
			&doc.ID,
			&doc.SourceID,
			&doc.ExternalID,
			&path,
			&title,
			&mimeType,
			&metadataJSON,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&indexedAt,
		)
		if err != nil {
			return nil, err
		}

		doc.Path = path.String
		doc.Title = title.String
		doc.MimeType = mimeType.String

		if indexedAt.Valid {
			doc.IndexedAt = indexedAt.Time
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
				return nil, err
			}
		}
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}

		docs = append(docs, &doc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return docs, nil
}

// Delete deletes a document
func (s *DocumentStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM documents WHERE id = $1`
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

// DeleteBySource deletes all documents for a source
func (s *DocumentStore) DeleteBySource(ctx context.Context, sourceID string) error {
	query := `DELETE FROM documents WHERE source_id = $1`
	_, err := s.db.ExecContext(ctx, query, sourceID)
	return err
}

// DeleteBatch deletes multiple documents by ID
func (s *DocumentStore) DeleteBatch(ctx context.Context, ids []string) error {
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

	query := `DELETE FROM documents WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// Count returns total document count
func (s *DocumentStore) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM documents`
	var count int
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// CountBySource returns document count for a source
func (s *DocumentStore) CountBySource(ctx context.Context, sourceID string) (int, error) {
	query := `SELECT COUNT(*) FROM documents WHERE source_id = $1`
	var count int
	err := s.db.QueryRowContext(ctx, query, sourceID).Scan(&count)
	return count, err
}

// ListExternalIDs returns all external IDs for a source (for diff sync)
func (s *DocumentStore) ListExternalIDs(ctx context.Context, sourceID string) ([]string, error) {
	query := `SELECT external_id FROM documents WHERE source_id = $1`

	rows, err := s.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

// DeleteBySourceAndContainer deletes all documents for a specific container within a source
func (s *DocumentStore) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	query := `DELETE FROM documents WHERE source_id = $1 AND metadata->>'container_id' = $2`
	_, err := s.db.ExecContext(ctx, query, sourceID, containerID)
	return err
}
