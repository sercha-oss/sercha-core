package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// SyncEventRepository handles sync event logging and audit trails
type SyncEventRepository interface {
	// Save logs a sync event for audit tracking
	Save(ctx context.Context, event *domain.SyncEvent) error

	// List retrieves recent sync events for a team
	// Returns up to limit most recent events, ordered by created_at desc
	List(ctx context.Context, teamID string, limit int) ([]*domain.SyncEvent, error)

	// ListBySource retrieves recent sync events for a specific source
	// Returns up to limit most recent events, ordered by created_at desc
	ListBySource(ctx context.Context, sourceID string, limit int) ([]*domain.SyncEvent, error)
}
