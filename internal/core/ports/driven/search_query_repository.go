package driven

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// SearchQueryRepository handles search query logging and analytics
type SearchQueryRepository interface {
	// Save logs a search query for analytics tracking
	Save(ctx context.Context, query *domain.SearchQuery) error

	// GetSearchHistory retrieves recent search queries
	// Returns up to limit most recent searches, ordered by created_at desc
	GetSearchHistory(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error)

	// GetSearchAnalytics computes aggregated search analytics for a time period
	GetSearchAnalytics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error)

	// GetSearchMetrics computes performance metrics for a time period
	GetSearchMetrics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error)
}
