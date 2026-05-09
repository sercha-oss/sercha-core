package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// CapabilityStore persists per-team capability toggle preferences.
//
// Storage is conceptually one row per (team, capability_type) toggle.
// Implementations expose this as a flat domain.CapabilityPreferences map
// keyed on CapabilityType, so service-side code stays generic — there are
// no typed columns per capability.
//
// Capabilities absent from the persisted set fall back to the descriptor's
// DefaultEnabled at resolution time. Implementations MUST NOT manufacture
// rows for capabilities the operator has not explicitly toggled — the
// difference between "explicitly true", "explicitly false", and "default"
// is meaningful.
type CapabilityStore interface {
	// GetPreferences returns the persisted preferences for a team. Empty
	// preferences (no rows) are not an error — the returned struct's
	// Toggles map is empty and consumers fall back to descriptor defaults.
	GetPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error)

	// SetToggles applies a partial update: each entry in toggles is
	// upserted as one row keyed on (team_id, capability_type). Toggles
	// not present in the map are left unchanged in storage.
	//
	// The empty map is a valid input (no-op). Implementations MUST be
	// idempotent — repeating a SetToggles call with the same input is
	// safe and produces the same end state.
	SetToggles(ctx context.Context, teamID string, toggles map[domain.CapabilityType]bool) error
}
