package domain

import "time"

// SearchQuery represents a logged search query for analytics
// This is an entity (has identity) used to track search usage and performance
type SearchQuery struct {
	// ID is the unique identifier for this search query log entry
	ID string `json:"id"`

	// TeamID is the team that performed the search
	TeamID string `json:"team_id"`

	// UserID is the user who performed the search
	UserID string `json:"user_id"`

	// Query is the search text that was submitted
	Query string `json:"query"`

	// Mode is the search mode used (hybrid, text, semantic)
	Mode SearchMode `json:"mode"`

	// ResultCount is the number of results returned
	ResultCount int `json:"result_count"`

	// Duration is how long the search took to execute (in nanoseconds)
	Duration int64 `json:"duration"`

	// SourceIDs are the source IDs that were filtered (if any)
	SourceIDs []string `json:"source_ids,omitempty"`

	// Filters are the additional filters applied
	HasFilters bool `json:"has_filters"`

	// CreatedAt is when the search was performed
	CreatedAt time.Time `json:"created_at"`
}

// NewSearchQuery creates a new search query log entry
func NewSearchQuery(teamID, userID, query string, mode SearchMode, resultCount int, duration time.Duration) *SearchQuery {
	return &SearchQuery{
		ID:          GenerateID(),
		TeamID:      teamID,
		UserID:      userID,
		Query:       query,
		Mode:        mode,
		ResultCount: resultCount,
		Duration:    duration.Nanoseconds(),
		CreatedAt:   time.Now(),
	}
}

// GetDuration returns the duration as a time.Duration
func (sq *SearchQuery) GetDuration() time.Duration {
	return time.Duration(sq.Duration)
}

// WithSourceFilters sets the source IDs filter
func (sq *SearchQuery) WithSourceFilters(sourceIDs []string) *SearchQuery {
	sq.SourceIDs = sourceIDs
	if len(sourceIDs) > 0 {
		sq.HasFilters = true
	}
	return sq
}

// WithFilters marks that additional filters were applied
func (sq *SearchQuery) WithFilters(hasFilters bool) *SearchQuery {
	sq.HasFilters = hasFilters
	return sq
}

// SearchAnalytics represents aggregated search analytics data
// This is a value object (no identity) representing computed statistics
type SearchAnalytics struct {
	// TotalSearches is the total number of searches in the period
	TotalSearches int64 `json:"total_searches"`

	// UniqueUsers is the number of unique users who searched
	UniqueUsers int64 `json:"unique_users"`

	// AverageDuration is the average search duration in milliseconds
	AverageDuration float64 `json:"average_duration_ms"`

	// AverageResults is the average number of results per search
	AverageResults float64 `json:"average_results"`

	// TopQueries are the most common search queries
	TopQueries []QueryFrequency `json:"top_queries"`

	// SearchesByMode shows the breakdown by search mode
	SearchesByMode map[SearchMode]int64 `json:"searches_by_mode"`

	// Period is the time range for these analytics
	Period AnalyticsPeriod `json:"period"`
}

// QueryFrequency represents how often a query appears
type QueryFrequency struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// AnalyticsPeriod represents the time range for analytics
type AnalyticsPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// SearchMetrics represents performance metrics for searches
// This is a value object for monitoring search performance
type SearchMetrics struct {
	// FastSearches is the number of searches under 100ms
	FastSearches int64 `json:"fast_searches"`

	// MediumSearches is the number of searches 100ms-500ms
	MediumSearches int64 `json:"medium_searches"`

	// SlowSearches is the number of searches over 500ms
	SlowSearches int64 `json:"slow_searches"`

	// P50Duration is the 50th percentile search duration in milliseconds
	P50Duration float64 `json:"p50_duration_ms"`

	// P95Duration is the 95th percentile search duration in milliseconds
	P95Duration float64 `json:"p95_duration_ms"`

	// P99Duration is the 99th percentile search duration in milliseconds
	P99Duration float64 `json:"p99_duration_ms"`

	// ZeroResultSearches is the number of searches that returned no results
	ZeroResultSearches int64 `json:"zero_result_searches"`

	// Period is the time range for these metrics
	Period AnalyticsPeriod `json:"period"`
}

// NewAnalyticsPeriod creates a new analytics period
func NewAnalyticsPeriod(start, end time.Time) AnalyticsPeriod {
	return AnalyticsPeriod{
		Start: start,
		End:   end,
	}
}

// Last24Hours creates a period for the last 24 hours
func Last24Hours() AnalyticsPeriod {
	now := time.Now()
	return AnalyticsPeriod{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}
}

// Last7Days creates a period for the last 7 days
func Last7Days() AnalyticsPeriod {
	now := time.Now()
	return AnalyticsPeriod{
		Start: now.Add(-7 * 24 * time.Hour),
		End:   now,
	}
}

// Last30Days creates a period for the last 30 days
func Last30Days() AnalyticsPeriod {
	now := time.Now()
	return AnalyticsPeriod{
		Start: now.Add(-30 * 24 * time.Hour),
		End:   now,
	}
}
