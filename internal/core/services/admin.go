package services

import (
	"context"
	"fmt"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Ensure adminService implements AdminService
var _ driving.AdminService = (*adminService)(nil)

// adminService implements the AdminService interface
type adminService struct {
	taskQueue             driven.TaskQueue
	schedulerStore        driven.SchedulerStore
	searchQueryRepository driven.SearchQueryRepository
	sourceStore           driven.SourceStore
}

// NewAdminService creates a new AdminService
func NewAdminService(
	taskQueue driven.TaskQueue,
	schedulerStore driven.SchedulerStore,
	searchQueryRepository driven.SearchQueryRepository,
	sourceStore driven.SourceStore,
) driving.AdminService {
	return &adminService{
		taskQueue:             taskQueue,
		schedulerStore:        schedulerStore,
		searchQueryRepository: searchQueryRepository,
		sourceStore:           sourceStore,
	}
}

// ListJobs retrieves job execution history with filtering and pagination
func (s *adminService) ListJobs(ctx context.Context, teamID string, req driving.ListJobsRequest) (*domain.JobHistory, error) {
	// Apply defaults
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Build filter
	filter := driven.TaskFilter{
		TeamID: teamID,
		Status: req.Status,
		Type:   req.Type,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	// Get tasks
	tasks, err := s.taskQueue.ListTasks(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Get total count for pagination
	totalCount, err := s.taskQueue.CountTasks(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("count tasks: %w", err)
	}

	// Build job history
	return domain.NewJobHistory(tasks, totalCount, req.Limit), nil
}

// GetUpcomingJobs retrieves pending tasks and scheduled task configurations
func (s *adminService) GetUpcomingJobs(ctx context.Context, teamID string) (*domain.UpcomingJobs, error) {
	// Get pending tasks
	pendingFilter := driven.TaskFilter{
		TeamID: teamID,
		Status: domain.TaskStatusPending,
		Limit:  20, // Limit to most recent 20 pending tasks
	}
	pendingTasks, err := s.taskQueue.ListTasks(ctx, pendingFilter)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	// Get scheduled tasks
	scheduledTasks, err := s.schedulerStore.ListScheduledTasks(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("list scheduled tasks: %w", err)
	}

	// Build upcoming jobs
	return domain.NewUpcomingJobs(pendingTasks, scheduledTasks), nil
}

// GetJob retrieves detailed information about a specific job
func (s *adminService) GetJob(ctx context.Context, teamID string, jobID string) (*domain.JobDetail, error) {
	// Get the task
	task, err := s.taskQueue.GetTask(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	// Verify task belongs to team
	if task.TeamID != teamID {
		return nil, domain.ErrNotFound
	}

	// Build job detail
	jobDetail := domain.NewJobDetail(task)

	// Enrich with source name if it's a sync_source task
	if task.Type == domain.TaskTypeSyncSource {
		sourceID := task.SourceID()
		if sourceID != "" {
			source, err := s.sourceStore.Get(ctx, sourceID)
			if err == nil && source != nil {
				jobDetail = jobDetail.WithSourceName(source.Name)
			}
		}
	}

	// TODO: Add retry history from task history table (future enhancement)
	// TODO: Add execution logs from logging system (future enhancement)

	return jobDetail, nil
}

// GetJobStats computes aggregated job statistics for a time period
func (s *adminService) GetJobStats(ctx context.Context, teamID string, period driving.JobStatsPeriod) (*domain.JobStats, error) {
	// Convert period to analytics period
	analyticsPeriod := s.convertJobStatsPeriod(period)

	// Get job stats from task queue
	stats, err := s.taskQueue.GetJobStats(ctx, teamID, analyticsPeriod)
	if err != nil {
		return nil, fmt.Errorf("get job stats: %w", err)
	}

	return stats, nil
}

// GetSearchAnalytics computes search usage analytics for a time period
func (s *adminService) GetSearchAnalytics(ctx context.Context, teamID string, period driving.SearchAnalyticsPeriod) (*domain.SearchAnalytics, error) {
	// Convert period to analytics period
	analyticsPeriod := s.convertSearchAnalyticsPeriod(period)

	// Get search analytics from repository
	analytics, err := s.searchQueryRepository.GetSearchAnalytics(ctx, teamID, analyticsPeriod)
	if err != nil {
		return nil, fmt.Errorf("get search analytics: %w", err)
	}

	return analytics, nil
}

// GetSearchHistory retrieves recent search queries
func (s *adminService) GetSearchHistory(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
	// Apply defaults
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Get search history from repository
	history, err := s.searchQueryRepository.GetSearchHistory(ctx, teamID, limit)
	if err != nil {
		return nil, fmt.Errorf("get search history: %w", err)
	}

	return history, nil
}

// GetSearchMetrics computes search performance metrics for a time period
func (s *adminService) GetSearchMetrics(ctx context.Context, teamID string, period driving.SearchAnalyticsPeriod) (*domain.SearchMetrics, error) {
	// Convert period to analytics period
	analyticsPeriod := s.convertSearchAnalyticsPeriod(period)

	// Get search metrics from repository
	metrics, err := s.searchQueryRepository.GetSearchMetrics(ctx, teamID, analyticsPeriod)
	if err != nil {
		return nil, fmt.Errorf("get search metrics: %w", err)
	}

	return metrics, nil
}

// TriggerReindex creates tasks to reindex sources
func (s *adminService) TriggerReindex(ctx context.Context, teamID string, req driving.TriggerReindexRequest) ([]string, error) {
	var sourcesToReindex []*domain.Source
	var err error

	// Determine which sources to reindex
	if len(req.SourceIDs) > 0 {
		// Reindex specific sources
		for _, sourceID := range req.SourceIDs {
			source, err := s.sourceStore.Get(ctx, sourceID)
			if err != nil {
				return nil, fmt.Errorf("get source %s: %w", sourceID, err)
			}
			if !source.Enabled {
				return nil, fmt.Errorf("source %s is disabled", sourceID)
			}
			sourcesToReindex = append(sourcesToReindex, source)
		}
	} else {
		// Reindex all enabled sources
		sourcesToReindex, err = s.sourceStore.ListEnabled(ctx)
		if err != nil {
			return nil, fmt.Errorf("list enabled sources: %w", err)
		}
	}

	if len(sourcesToReindex) == 0 {
		return nil, fmt.Errorf("no sources to reindex")
	}

	// Create sync tasks for each source
	tasks := make([]*domain.Task, 0, len(sourcesToReindex))
	for _, source := range sourcesToReindex {
		task := domain.NewSyncSourceTask(teamID, source.ID)
		task.Priority = req.Priority
		tasks = append(tasks, task)
	}

	// Enqueue all tasks as a batch
	if err := s.taskQueue.EnqueueBatch(ctx, tasks); err != nil {
		return nil, fmt.Errorf("enqueue reindex tasks: %w", err)
	}

	// Collect task IDs
	taskIDs := make([]string, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	return taskIDs, nil
}

// convertJobStatsPeriod converts the API period to domain AnalyticsPeriod
func (s *adminService) convertJobStatsPeriod(period driving.JobStatsPeriod) domain.AnalyticsPeriod {
	switch period {
	case driving.JobStatsPeriod24Hours:
		return domain.Last24Hours()
	case driving.JobStatsPeriod7Days:
		return domain.Last7Days()
	case driving.JobStatsPeriod30Days:
		return domain.Last30Days()
	default:
		return domain.Last24Hours()
	}
}

// convertSearchAnalyticsPeriod converts the API period to domain AnalyticsPeriod
func (s *adminService) convertSearchAnalyticsPeriod(period driving.SearchAnalyticsPeriod) domain.AnalyticsPeriod {
	switch period {
	case driving.SearchAnalyticsPeriod24Hours:
		return domain.Last24Hours()
	case driving.SearchAnalyticsPeriod7Days:
		return domain.Last7Days()
	case driving.SearchAnalyticsPeriod30Days:
		return domain.Last30Days()
	default:
		return domain.Last24Hours()
	}
}
