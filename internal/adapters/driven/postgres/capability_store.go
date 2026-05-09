package postgres

import (
	"context"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.CapabilityStore = (*CapabilityStore)(nil)

// CapabilityStore implements driven.CapabilityStore against the per-row
// capability_preferences table. Each (team_id, capability_type) is one
// row; capabilities absent from the rowset have no explicit preference
// and fall back to descriptor defaults at resolution time.
type CapabilityStore struct {
	db *DB
}

// NewCapabilityStore creates a new CapabilityStore.
func NewCapabilityStore(db *DB) *CapabilityStore {
	return &CapabilityStore{db: db}
}

// GetPreferences returns every persisted toggle for the team. Empty result
// is not an error — the returned preferences's Toggles map is empty and
// callers fall back to descriptor defaults.
func (s *CapabilityStore) GetPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error) {
	q := `
		SELECT capability_type, enabled, updated_at
		FROM capability_preferences
		WHERE team_id = $1
	`
	rows, err := s.db.QueryContext(ctx, q, teamID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	prefs := domain.DefaultCapabilityPreferences(teamID)
	prefs.Toggles = map[domain.CapabilityType]bool{}
	var maxUpdated time.Time
	for rows.Next() {
		var capType string
		var enabled bool
		var updatedAt time.Time
		if err := rows.Scan(&capType, &enabled, &updatedAt); err != nil {
			return nil, err
		}
		prefs.Toggles[domain.CapabilityType(capType)] = enabled
		if updatedAt.After(maxUpdated) {
			maxUpdated = updatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !maxUpdated.IsZero() {
		prefs.UpdatedAt = maxUpdated
	}
	return prefs, nil
}

// SetToggles upserts a partial set of toggles for the team. Toggles not
// present in the input are left unchanged in storage.
func (s *CapabilityStore) SetToggles(ctx context.Context, teamID string, toggles map[domain.CapabilityType]bool) error {
	if len(toggles) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO capability_preferences (team_id, capability_type, enabled, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (team_id, capability_type) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			updated_at = NOW()
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for capType, enabled := range toggles {
		if _, err := stmt.ExecContext(ctx, teamID, string(capType), enabled); err != nil {
			return err
		}
	}
	return tx.Commit()
}
