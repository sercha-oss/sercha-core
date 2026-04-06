package driving

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// ListJobsRequest represents filtering options for job history
type ListJobsRequest struct {
	// Status filters by task status (optional)
	Status domain.TaskStatus `json:"status,omitempty"`

	// Type filters by task type (optional)
	Type domain.TaskType `json:"type,omitempty"`

	// Limit is the maximum number of jobs to return
	Limit int `json:"limit"`

	// Offset is the number of jobs to skip (for pagination)
	Offset int `json:"offset"`
}

// JobStatsPeriod represents the time period for job statistics
type JobStatsPeriod string

const (
	JobStatsPeriod24Hours JobStatsPeriod = "24h"
	JobStatsPeriod7Days   JobStatsPeriod = "7d"
	JobStatsPeriod30Days  JobStatsPeriod = "30d"
)

// SearchAnalyticsPeriod represents the time period for search analytics
type SearchAnalyticsPeriod string

const (
	SearchAnalyticsPeriod24Hours SearchAnalyticsPeriod = "24h"
	SearchAnalyticsPeriod7Days   SearchAnalyticsPeriod = "7d"
	SearchAnalyticsPeriod30Days  SearchAnalyticsPeriod = "30d"
)

// TriggerReindexRequest represents options for triggering a full reindex
type TriggerReindexRequest struct {
	// SourceIDs optionally specifies which sources to reindex
	// If empty, all enabled sources are reindexed
	SourceIDs []string `json:"source_ids,omitempty"`

	// Priority sets the task priority (-100 to 100)
	Priority int `json:"priority"`
}

// AdminService provides administrative operations for the dashboard
// This includes job management, search analytics, and system operations
type AdminService interface {
	// Job Management

	// ListJobs retrieves job execution history with filtering and pagination
	ListJobs(ctx context.Context, teamID string, req ListJobsRequest) (*domain.JobHistory, error)

	// GetUpcomingJobs retrieves pending tasks and scheduled task configurations
	GetUpcomingJobs(ctx context.Context, teamID string) (*domain.UpcomingJobs, error)

	// GetJob retrieves detailed information about a specific job
	GetJob(ctx context.Context, teamID string, jobID string) (*domain.JobDetail, error)

	// GetJobStats computes aggregated job statistics for a time period
	GetJobStats(ctx context.Context, teamID string, period JobStatsPeriod) (*domain.JobStats, error)

	// Search Analytics

	// GetSearchAnalytics computes search usage analytics for a time period
	GetSearchAnalytics(ctx context.Context, teamID string, period SearchAnalyticsPeriod) (*domain.SearchAnalytics, error)

	// GetSearchHistory retrieves recent search queries
	GetSearchHistory(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error)

	// GetSearchMetrics computes search performance metrics for a time period
	GetSearchMetrics(ctx context.Context, teamID string, period SearchAnalyticsPeriod) (*domain.SearchMetrics, error)

	// Operations

	// TriggerReindex creates tasks to reindex sources
	// Returns the task IDs that were created
	TriggerReindex(ctx context.Context, teamID string, req TriggerReindexRequest) ([]string, error)
}
