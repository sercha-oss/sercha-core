package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SearchQueryRepository = (*SearchQueryRepository)(nil)

// SearchQueryRepository implements driven.SearchQueryRepository using PostgreSQL
type SearchQueryRepository struct {
	db *DB
}

// NewSearchQueryRepository creates a new SearchQueryRepository
func NewSearchQueryRepository(db *DB) *SearchQueryRepository {
	return &SearchQueryRepository{db: db}
}

// Save logs a search query for analytics tracking
func (r *SearchQueryRepository) Save(ctx context.Context, query *domain.SearchQuery) error {
	sourceIDsJSON, err := json.Marshal(query.SourceIDs)
	if err != nil {
		return fmt.Errorf("marshal source_ids: %w", err)
	}

	q := `
		INSERT INTO search_queries (
			id, team_id, user_id, query, mode, result_count, duration_ns,
			source_ids, has_filters, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.ExecContext(ctx, q,
		query.ID,
		query.TeamID,
		query.UserID,
		query.Query,
		string(query.Mode),
		query.ResultCount,
		query.Duration,
		sourceIDsJSON,
		query.HasFilters,
		query.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert search query: %w", err)
	}

	return nil
}

// GetSearchHistory retrieves recent search queries
func (r *SearchQueryRepository) GetSearchHistory(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := `
		SELECT id, team_id, user_id, query, mode, result_count, duration_ns,
		       source_ids, has_filters, created_at
		FROM search_queries
		WHERE team_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, q, teamID, limit)
	if err != nil {
		return nil, fmt.Errorf("query search history: %w", err)
	}
	defer rows.Close()

	var queries []*domain.SearchQuery
	for rows.Next() {
		var sq domain.SearchQuery
		var sourceIDsJSON []byte
		var mode string

		err := rows.Scan(
			&sq.ID,
			&sq.TeamID,
			&sq.UserID,
			&sq.Query,
			&mode,
			&sq.ResultCount,
			&sq.Duration,
			&sourceIDsJSON,
			&sq.HasFilters,
			&sq.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search query: %w", err)
		}

		sq.Mode = domain.SearchMode(mode)

		if len(sourceIDsJSON) > 0 {
			if err := json.Unmarshal(sourceIDsJSON, &sq.SourceIDs); err != nil {
				return nil, fmt.Errorf("unmarshal source_ids: %w", err)
			}
		}

		queries = append(queries, &sq)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search queries: %w", err)
	}

	return queries, nil
}

// GetSearchAnalytics computes aggregated search analytics for a time period
func (r *SearchQueryRepository) GetSearchAnalytics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
	analytics := &domain.SearchAnalytics{
		Period:         period,
		SearchesByMode: make(map[domain.SearchMode]int64),
	}

	// Get total searches and aggregate metrics
	// Use COALESCE to handle NULL from AVG when no rows exist
	q := `
		SELECT
			COUNT(*) as total_searches,
			COUNT(DISTINCT user_id) as unique_users,
			COALESCE(AVG(duration_ns / 1000000.0), 0) as avg_duration_ms,
			COALESCE(AVG(result_count), 0) as avg_results
		FROM search_queries
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	err := r.db.QueryRowContext(ctx, q, teamID, period.Start, period.End).Scan(
		&analytics.TotalSearches,
		&analytics.UniqueUsers,
		&analytics.AverageDuration,
		&analytics.AverageResults,
	)
	if err != nil {
		return nil, fmt.Errorf("query aggregate analytics: %w", err)
	}

	// Get searches by mode
	modeQuery := `
		SELECT mode, COUNT(*) as count
		FROM search_queries
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY mode
	`

	rows, err := r.db.QueryContext(ctx, modeQuery, teamID, period.Start, period.End)
	if err != nil {
		return nil, fmt.Errorf("query searches by mode: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mode string
		var count int64
		if err := rows.Scan(&mode, &count); err != nil {
			return nil, fmt.Errorf("scan mode count: %w", err)
		}
		analytics.SearchesByMode[domain.SearchMode(mode)] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mode counts: %w", err)
	}

	// Get top queries
	topQuery := `
		SELECT query, COUNT(*) as count
		FROM search_queries
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		  AND query != ''
		GROUP BY query
		ORDER BY count DESC
		LIMIT 10
	`

	topRows, err := r.db.QueryContext(ctx, topQuery, teamID, period.Start, period.End)
	if err != nil {
		return nil, fmt.Errorf("query top queries: %w", err)
	}
	defer topRows.Close()

	for topRows.Next() {
		var qf domain.QueryFrequency
		if err := topRows.Scan(&qf.Query, &qf.Count); err != nil {
			return nil, fmt.Errorf("scan query frequency: %w", err)
		}
		analytics.TopQueries = append(analytics.TopQueries, qf)
	}

	if err := topRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top queries: %w", err)
	}

	return analytics, nil
}

// GetSearchMetrics computes performance metrics for a time period
func (r *SearchQueryRepository) GetSearchMetrics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error) {
	metrics := &domain.SearchMetrics{
		Period: period,
	}

	// Get speed distribution and zero results
	q := `
		SELECT
			COUNT(*) FILTER (WHERE duration_ns < 100000000) as fast_searches,
			COUNT(*) FILTER (WHERE duration_ns >= 100000000 AND duration_ns < 500000000) as medium_searches,
			COUNT(*) FILTER (WHERE duration_ns >= 500000000) as slow_searches,
			COUNT(*) FILTER (WHERE result_count = 0) as zero_result_searches
		FROM search_queries
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	err := r.db.QueryRowContext(ctx, q, teamID, period.Start, period.End).Scan(
		&metrics.FastSearches,
		&metrics.MediumSearches,
		&metrics.SlowSearches,
		&metrics.ZeroResultSearches,
	)
	if err != nil {
		return nil, fmt.Errorf("query search metrics: %w", err)
	}

	// Get percentiles using PostgreSQL's PERCENTILE_CONT
	percentileQuery := `
		SELECT
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_ns / 1000000.0) as p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ns / 1000000.0) as p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_ns / 1000000.0) as p99
		FROM search_queries
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	var p50, p95, p99 sql.NullFloat64
	err = r.db.QueryRowContext(ctx, percentileQuery, teamID, period.Start, period.End).Scan(&p50, &p95, &p99)
	if err != nil {
		return nil, fmt.Errorf("query percentiles: %w", err)
	}

	if p50.Valid {
		metrics.P50Duration = p50.Float64
	}
	if p95.Valid {
		metrics.P95Duration = p95.Float64
	}
	if p99.Valid {
		metrics.P99Duration = p99.Float64
	}

	return metrics, nil
}
