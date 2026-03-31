package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SettingsStore = (*SettingsStore)(nil)

// SettingsStore implements driven.SettingsStore using PostgreSQL
type SettingsStore struct {
	db *DB
}

// NewSettingsStore creates a new SettingsStore
func NewSettingsStore(db *DB) *SettingsStore {
	return &SettingsStore{db: db}
}

// GetSettings retrieves settings for a team
// Note: AI configuration is managed via AISettings (ai_settings table), not here
func (s *SettingsStore) GetSettings(ctx context.Context, teamID string) (*domain.Settings, error) {
	query := `
		SELECT team_id, default_search_mode, results_per_page, max_results_per_page,
			   sync_interval_minutes, sync_enabled, semantic_search_enabled,
			   auto_suggest_enabled, updated_at, updated_by
		FROM settings
		WHERE team_id = $1
	`

	var settings domain.Settings
	var updatedBy sql.NullString

	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&settings.TeamID,
		&settings.DefaultSearchMode,
		&settings.ResultsPerPage,
		&settings.MaxResultsPerPage,
		&settings.SyncIntervalMinutes,
		&settings.SyncEnabled,
		&settings.SemanticSearchEnabled,
		&settings.AutoSuggestEnabled,
		&settings.UpdatedAt,
		&updatedBy,
	)
	if err == sql.ErrNoRows {
		// Return default settings if not found
		return domain.DefaultSettings(teamID), nil
	}
	if err != nil {
		return nil, err
	}

	settings.UpdatedBy = updatedBy.String

	return &settings, nil
}

// SaveSettings persists team settings
// Note: AI configuration is managed via SaveAISettings, not here
func (s *SettingsStore) SaveSettings(ctx context.Context, settings *domain.Settings) error {
	query := `
		INSERT INTO settings (team_id, default_search_mode, results_per_page, max_results_per_page,
							  sync_interval_minutes, sync_enabled, semantic_search_enabled,
							  auto_suggest_enabled, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (team_id) DO UPDATE SET
			default_search_mode = EXCLUDED.default_search_mode,
			results_per_page = EXCLUDED.results_per_page,
			max_results_per_page = EXCLUDED.max_results_per_page,
			sync_interval_minutes = EXCLUDED.sync_interval_minutes,
			sync_enabled = EXCLUDED.sync_enabled,
			semantic_search_enabled = EXCLUDED.semantic_search_enabled,
			auto_suggest_enabled = EXCLUDED.auto_suggest_enabled,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`

	settings.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, query,
		settings.TeamID,
		string(settings.DefaultSearchMode),
		settings.ResultsPerPage,
		settings.MaxResultsPerPage,
		settings.SyncIntervalMinutes,
		settings.SyncEnabled,
		settings.SemanticSearchEnabled,
		settings.AutoSuggestEnabled,
		settings.UpdatedAt,
		settings.UpdatedBy,
	)
	return err
}

// GetAISettings retrieves AI-specific settings for a team
// Note: API keys and base URLs are NOT stored in database - they come from environment variables
func (s *SettingsStore) GetAISettings(ctx context.Context, teamID string) (*domain.AISettings, error) {
	query := `
		SELECT team_id, embedding_provider, embedding_model,
			   llm_provider, llm_model, updated_at
		FROM ai_settings
		WHERE team_id = $1
	`

	var settings domain.AISettings
	var embProvider, embModel sql.NullString
	var llmProvider, llmModel sql.NullString

	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&settings.TeamID,
		&embProvider,
		&embModel,
		&llmProvider,
		&llmModel,
		&settings.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Return empty settings if not found
		return &domain.AISettings{TeamID: teamID}, nil
	}
	if err != nil {
		return nil, err
	}

	if embProvider.Valid {
		settings.Embedding.Provider = domain.AIProvider(embProvider.String)
	}
	settings.Embedding.Model = embModel.String

	if llmProvider.Valid {
		settings.LLM.Provider = domain.AIProvider(llmProvider.String)
	}
	settings.LLM.Model = llmModel.String

	return &settings, nil
}

// SaveAISettings persists AI-specific settings
// Note: Only provider and model are stored - API keys and base URLs come from environment
func (s *SettingsStore) SaveAISettings(ctx context.Context, teamID string, settings *domain.AISettings) error {
	query := `
		INSERT INTO ai_settings (team_id, embedding_provider, embedding_model,
								 llm_provider, llm_model, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (team_id) DO UPDATE SET
			embedding_provider = EXCLUDED.embedding_provider,
			embedding_model = EXCLUDED.embedding_model,
			llm_provider = EXCLUDED.llm_provider,
			llm_model = EXCLUDED.llm_model,
			updated_at = EXCLUDED.updated_at
	`

	settings.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, query,
		teamID,
		string(settings.Embedding.Provider),
		settings.Embedding.Model,
		string(settings.LLM.Provider),
		settings.LLM.Model,
		settings.UpdatedAt,
	)
	return err
}
