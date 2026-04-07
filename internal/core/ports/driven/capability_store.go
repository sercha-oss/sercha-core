package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// CapabilityStore manages capability preferences persistence
type CapabilityStore interface {
	// GetPreferences retrieves capability preferences for a team.
	// Returns error if team not found or database error occurs.
	GetPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error)

	// SavePreferences persists capability preferences for a team.
	// Creates new preferences if they don't exist, updates if they do.
	SavePreferences(ctx context.Context, prefs *domain.CapabilityPreferences) error
}
