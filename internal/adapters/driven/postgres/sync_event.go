package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SyncEventRepository = (*SyncEventRepository)(nil)

// SyncEventRepository implements driven.SyncEventRepository using PostgreSQL
type SyncEventRepository struct {
	db *DB
}

// NewSyncEventRepository creates a new SyncEventRepository
func NewSyncEventRepository(db *DB) *SyncEventRepository {
	return &SyncEventRepository{db: db}
}

// Save logs a sync event for audit tracking
func (r *SyncEventRepository) Save(ctx context.Context, event *domain.SyncEvent) error {
	q := `
		INSERT INTO sync_events (
			id, team_id, source_id, source_name, provider_type, status,
			documents_added, documents_updated, documents_deleted,
			chunks_indexed, error_count, error_message, duration_seconds,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, q,
		event.ID,
		event.TeamID,
		event.SourceID,
		event.SourceName,
		string(event.ProviderType),
		string(event.Status),
		event.DocumentsAdded,
		event.DocumentsUpdated,
		event.DocumentsDeleted,
		event.ChunksIndexed,
		event.ErrorCount,
		event.ErrorMessage,
		event.DurationSeconds,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert sync event: %w", err)
	}

	return nil
}

// List retrieves recent sync events for a team
func (r *SyncEventRepository) List(ctx context.Context, teamID string, limit int) ([]*domain.SyncEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := `
		SELECT id, team_id, source_id, source_name, provider_type, status,
		       documents_added, documents_updated, documents_deleted,
		       chunks_indexed, error_count, error_message, duration_seconds,
		       created_at
		FROM sync_events
		WHERE team_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, q, teamID, limit)
	if err != nil {
		return nil, fmt.Errorf("query sync events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []*domain.SyncEvent
	for rows.Next() {
		var se domain.SyncEvent
		var providerType string
		var status string
		var errorMessage sql.NullString

		err := rows.Scan(
			&se.ID,
			&se.TeamID,
			&se.SourceID,
			&se.SourceName,
			&providerType,
			&status,
			&se.DocumentsAdded,
			&se.DocumentsUpdated,
			&se.DocumentsDeleted,
			&se.ChunksIndexed,
			&se.ErrorCount,
			&errorMessage,
			&se.DurationSeconds,
			&se.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sync event: %w", err)
		}

		se.ProviderType = domain.ProviderType(providerType)
		se.Status = domain.SyncStatus(status)
		if errorMessage.Valid {
			se.ErrorMessage = errorMessage.String
		}

		events = append(events, &se)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync events: %w", err)
	}

	return events, nil
}

// ListBySource retrieves recent sync events for a specific source
func (r *SyncEventRepository) ListBySource(ctx context.Context, sourceID string, limit int) ([]*domain.SyncEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := `
		SELECT id, team_id, source_id, source_name, provider_type, status,
		       documents_added, documents_updated, documents_deleted,
		       chunks_indexed, error_count, error_message, duration_seconds,
		       created_at
		FROM sync_events
		WHERE source_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, q, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("query sync events by source: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []*domain.SyncEvent
	for rows.Next() {
		var se domain.SyncEvent
		var providerType string
		var status string
		var errorMessage sql.NullString

		err := rows.Scan(
			&se.ID,
			&se.TeamID,
			&se.SourceID,
			&se.SourceName,
			&providerType,
			&status,
			&se.DocumentsAdded,
			&se.DocumentsUpdated,
			&se.DocumentsDeleted,
			&se.ChunksIndexed,
			&se.ErrorCount,
			&errorMessage,
			&se.DurationSeconds,
			&se.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sync event: %w", err)
		}

		se.ProviderType = domain.ProviderType(providerType)
		se.Status = domain.SyncStatus(status)
		if errorMessage.Valid {
			se.ErrorMessage = errorMessage.String
		}

		events = append(events, &se)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync events by source: %w", err)
	}

	return events, nil
}
