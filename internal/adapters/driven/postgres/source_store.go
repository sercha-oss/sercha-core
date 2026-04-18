package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SourceStore = (*SourceStore)(nil)

// SourceStore implements driven.SourceStore using PostgreSQL
type SourceStore struct {
	db *DB
}

// NewSourceStore creates a new SourceStore
func NewSourceStore(db *DB) *SourceStore {
	return &SourceStore{db: db}
}

// Save creates or updates a source
func (s *SourceStore) Save(ctx context.Context, source *domain.Source) error {
	configJSON, err := json.Marshal(source.Config)
	if err != nil {
		return err
	}

	containersJSON, err := json.Marshal(source.Containers)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO sources (id, name, provider_type, config, enabled, created_at, updated_at, created_by, connection_id, selected_containers)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			provider_type = EXCLUDED.provider_type,
			config = EXCLUDED.config,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at,
			connection_id = EXCLUDED.connection_id,
			selected_containers = EXCLUDED.selected_containers
	`

	_, err = s.db.ExecContext(ctx, query,
		source.ID,
		source.Name,
		string(source.ProviderType),
		configJSON,
		source.Enabled,
		source.CreatedAt,
		source.UpdatedAt,
		source.CreatedBy,
		sql.NullString{String: source.ConnectionID, Valid: source.ConnectionID != ""},
		containersJSON,
	)
	return err
}

// Get retrieves a source by ID
func (s *SourceStore) Get(ctx context.Context, id string) (*domain.Source, error) {
	query := `
		SELECT id, name, provider_type, config, enabled, created_at, updated_at, created_by,
		       connection_id, selected_containers
		FROM sources
		WHERE id = $1
	`

	var source domain.Source
	var configJSON []byte
	var containersJSON []byte
	var createdBy, connectionID sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&source.ID,
		&source.Name,
		&source.ProviderType,
		&configJSON,
		&source.Enabled,
		&source.CreatedAt,
		&source.UpdatedAt,
		&createdBy,
		&connectionID,
		&containersJSON,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &source.Config); err != nil {
		return nil, err
	}
	if len(containersJSON) > 0 {
		if err := json.Unmarshal(containersJSON, &source.Containers); err != nil {
			return nil, err
		}
	}
	source.CreatedBy = createdBy.String
	source.ConnectionID = connectionID.String

	return &source, nil
}

// GetByName retrieves a source by name
func (s *SourceStore) GetByName(ctx context.Context, name string) (*domain.Source, error) {
	query := `
		SELECT id, name, provider_type, config, enabled, created_at, updated_at, created_by,
		       connection_id, selected_containers
		FROM sources
		WHERE name = $1
	`

	var source domain.Source
	var configJSON []byte
	var containersJSON []byte
	var createdBy, connectionID sql.NullString

	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&source.ID,
		&source.Name,
		&source.ProviderType,
		&configJSON,
		&source.Enabled,
		&source.CreatedAt,
		&source.UpdatedAt,
		&createdBy,
		&connectionID,
		&containersJSON,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &source.Config); err != nil {
		return nil, err
	}
	if len(containersJSON) > 0 {
		if err := json.Unmarshal(containersJSON, &source.Containers); err != nil {
			return nil, err
		}
	}
	source.CreatedBy = createdBy.String
	source.ConnectionID = connectionID.String

	return &source, nil
}

// List retrieves all sources
func (s *SourceStore) List(ctx context.Context) ([]*domain.Source, error) {
	query := `
		SELECT id, name, provider_type, config, enabled, created_at, updated_at, created_by,
		       connection_id, selected_containers
		FROM sources
		ORDER BY created_at DESC
	`

	return s.querySources(ctx, query)
}

// ListEnabled retrieves all enabled sources
func (s *SourceStore) ListEnabled(ctx context.Context) ([]*domain.Source, error) {
	query := `
		SELECT id, name, provider_type, config, enabled, created_at, updated_at, created_by,
		       connection_id, selected_containers
		FROM sources
		WHERE enabled = true
		ORDER BY created_at DESC
	`

	return s.querySources(ctx, query)
}

func (s *SourceStore) querySources(ctx context.Context, query string, args ...interface{}) ([]*domain.Source, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sources []*domain.Source
	for rows.Next() {
		var source domain.Source
		var configJSON []byte
		var containersJSON []byte
		var createdBy, connectionID sql.NullString

		err := rows.Scan(
			&source.ID,
			&source.Name,
			&source.ProviderType,
			&configJSON,
			&source.Enabled,
			&source.CreatedAt,
			&source.UpdatedAt,
			&createdBy,
			&connectionID,
			&containersJSON,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(configJSON, &source.Config); err != nil {
			return nil, err
		}
		if len(containersJSON) > 0 {
			if err := json.Unmarshal(containersJSON, &source.Containers); err != nil {
				return nil, err
			}
		}
		source.CreatedBy = createdBy.String
		source.ConnectionID = connectionID.String
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sources, nil
}

// Delete deletes a source
func (s *SourceStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM sources WHERE id = $1`
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

// SetEnabled updates the enabled status
func (s *SourceStore) SetEnabled(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE sources SET enabled = $1, updated_at = $2 WHERE id = $3`
	result, err := s.db.ExecContext(ctx, query, enabled, time.Now(), id)
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

// CountByConnection returns the number of sources using a connection
func (s *SourceStore) CountByConnection(ctx context.Context, connectionID string) (int, error) {
	query := `SELECT COUNT(*) FROM sources WHERE connection_id = $1`
	var count int
	err := s.db.QueryRowContext(ctx, query, connectionID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListByConnection returns sources using a connection
func (s *SourceStore) ListByConnection(ctx context.Context, connectionID string) ([]*domain.Source, error) {
	query := `
		SELECT id, name, provider_type, config, enabled, created_at, updated_at, created_by,
		       connection_id, selected_containers
		FROM sources
		WHERE connection_id = $1
		ORDER BY created_at DESC
	`

	return s.querySourcesWithConnection(ctx, query, connectionID)
}

// UpdateContainers updates the selected containers for a source
func (s *SourceStore) UpdateContainers(ctx context.Context, id string, containers []domain.Container) error {
	containersJSON, err := json.Marshal(containers)
	if err != nil {
		return err
	}

	query := `UPDATE sources SET selected_containers = $1, updated_at = $2 WHERE id = $3`
	result, err := s.db.ExecContext(ctx, query, containersJSON, time.Now(), id)
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

// querySourcesWithConnection is like querySources but includes connection fields
func (s *SourceStore) querySourcesWithConnection(ctx context.Context, query string, args ...interface{}) ([]*domain.Source, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sources []*domain.Source
	for rows.Next() {
		var source domain.Source
		var configJSON []byte
		var containersJSON []byte
		var createdBy, connectionID sql.NullString

		err := rows.Scan(
			&source.ID,
			&source.Name,
			&source.ProviderType,
			&configJSON,
			&source.Enabled,
			&source.CreatedAt,
			&source.UpdatedAt,
			&createdBy,
			&connectionID,
			&containersJSON,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(configJSON, &source.Config); err != nil {
			return nil, err
		}
		if len(containersJSON) > 0 {
			if err := json.Unmarshal(containersJSON, &source.Containers); err != nil {
				return nil, err
			}
		}
		source.CreatedBy = createdBy.String
		source.ConnectionID = connectionID.String
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sources, nil
}
