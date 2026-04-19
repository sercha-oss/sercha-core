package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.CapabilityStore = (*CapabilityStore)(nil)

// CapabilityStore implements driven.CapabilityStore using PostgreSQL
type CapabilityStore struct {
	db *DB
}

// NewCapabilityStore creates a new CapabilityStore
func NewCapabilityStore(db *DB) *CapabilityStore {
	return &CapabilityStore{db: db}
}

// GetPreferences retrieves capability preferences for a team
func (s *CapabilityStore) GetPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error) {
	query := `
		SELECT team_id, text_indexing_enabled, embedding_indexing_enabled,
		       bm25_search_enabled, vector_search_enabled,
		       query_expansion_enabled, query_rewriting_enabled, summarization_enabled,
		       updated_at
		FROM capability_preferences
		WHERE team_id = $1
	`

	var prefs domain.CapabilityPreferences

	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&prefs.TeamID,
		&prefs.TextIndexingEnabled,
		&prefs.EmbeddingIndexingEnabled,
		&prefs.BM25SearchEnabled,
		&prefs.VectorSearchEnabled,
		&prefs.QueryExpansionEnabled,
		&prefs.QueryRewritingEnabled,
		&prefs.SummarizationEnabled,
		&prefs.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Return default preferences if not found
		return domain.DefaultCapabilityPreferences(teamID), nil
	}
	if err != nil {
		return nil, err
	}

	return &prefs, nil
}

// SavePreferences persists capability preferences for a team
func (s *CapabilityStore) SavePreferences(ctx context.Context, prefs *domain.CapabilityPreferences) error {
	query := `
		INSERT INTO capability_preferences (team_id, text_indexing_enabled, embedding_indexing_enabled,
		                                     bm25_search_enabled, vector_search_enabled,
		                                     query_expansion_enabled, query_rewriting_enabled, summarization_enabled,
		                                     updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (team_id) DO UPDATE SET
			text_indexing_enabled = EXCLUDED.text_indexing_enabled,
			embedding_indexing_enabled = EXCLUDED.embedding_indexing_enabled,
			bm25_search_enabled = EXCLUDED.bm25_search_enabled,
			vector_search_enabled = EXCLUDED.vector_search_enabled,
			query_expansion_enabled = EXCLUDED.query_expansion_enabled,
			query_rewriting_enabled = EXCLUDED.query_rewriting_enabled,
			summarization_enabled = EXCLUDED.summarization_enabled,
			updated_at = EXCLUDED.updated_at
	`

	prefs.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, query,
		prefs.TeamID,
		prefs.TextIndexingEnabled,
		prefs.EmbeddingIndexingEnabled,
		prefs.BM25SearchEnabled,
		prefs.VectorSearchEnabled,
		prefs.QueryExpansionEnabled,
		prefs.QueryRewritingEnabled,
		prefs.SummarizationEnabled,
		prefs.UpdatedAt,
	)
	return err
}
