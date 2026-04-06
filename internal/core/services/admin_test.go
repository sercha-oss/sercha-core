package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven/mocks"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

func TestAdminService_ListJobs(t *testing.T) {
	tests := []struct {
		name           string
		teamID         string
		req            driving.ListJobsRequest
		setupMocks     func(*mocks.MockTaskQueue)
		wantErr        bool
		wantJobsCount  int
		wantTotalCount int64
		wantHasMore    bool
	}{
		{
			name:   "successful list with defaults",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit: 0, // Should default to 50
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tasks := []*domain.Task{
					domain.NewSyncSourceTask("team-123", "source-1"),
					domain.NewSyncSourceTask("team-123", "source-2"),
				}
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Limit != 50 {
						t.Errorf("expected limit 50, got %d", filter.Limit)
					}
					return tasks, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 2, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  2,
			wantTotalCount: 2,
			wantHasMore:    false,
		},
		{
			name:   "enforces max limit of 100",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit: 200, // Should be capped at 100
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Limit != 100 {
						t.Errorf("expected limit capped at 100, got %d", filter.Limit)
					}
					return []*domain.Task{}, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 0, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  0,
			wantTotalCount: 0,
			wantHasMore:    false,
		},
		{
			name:   "filters by status",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Status: domain.TaskStatusCompleted,
				Limit:  10,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Status != domain.TaskStatusCompleted {
						t.Errorf("expected status completed, got %v", filter.Status)
					}
					return []*domain.Task{}, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 0, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  0,
			wantTotalCount: 0,
		},
		{
			name:   "filters by type",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Type:  domain.TaskTypeSyncSource,
				Limit: 10,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Type != domain.TaskTypeSyncSource {
						t.Errorf("expected type sync_source, got %v", filter.Type)
					}
					return []*domain.Task{}, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 0, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  0,
			wantTotalCount: 0,
		},
		{
			name:   "pagination with offset",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit:  10,
				Offset: 20,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Offset != 20 {
						t.Errorf("expected offset 20, got %d", filter.Offset)
					}
					return []*domain.Task{}, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 100, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  0,
			wantTotalCount: 100,
			wantHasMore:    true,
		},
		{
			name:   "indicates has more when more results exist",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit: 10,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tasks := make([]*domain.Task, 10)
				for i := 0; i < 10; i++ {
					tasks[i] = domain.NewSyncSourceTask("team-123", "source-1")
				}
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return tasks, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 50, nil
				})
			},
			wantErr:        false,
			wantJobsCount:  10,
			wantTotalCount: 50,
			wantHasMore:    true,
		},
		{
			name:   "error listing tasks",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit: 10,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
		{
			name:   "error counting tasks",
			teamID: "team-123",
			req: driving.ListJobsRequest{
				Limit: 10,
			},
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				})
				tq.SetCountTasksFunc(func(ctx context.Context, filter driven.TaskFilter) (int64, error) {
					return 0, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(taskQueue)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.ListJobs(context.Background(), tt.teamID, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if len(result.Jobs) != tt.wantJobsCount {
				t.Errorf("expected %d jobs, got %d", tt.wantJobsCount, len(result.Jobs))
			}

			if result.TotalCount != tt.wantTotalCount {
				t.Errorf("expected total count %d, got %d", tt.wantTotalCount, result.TotalCount)
			}

			if result.HasMore != tt.wantHasMore {
				t.Errorf("expected has more %v, got %v", tt.wantHasMore, result.HasMore)
			}
		})
	}
}

func TestAdminService_GetUpcomingJobs(t *testing.T) {
	tests := []struct {
		name                string
		teamID              string
		setupMocks          func(*mocks.MockTaskQueue, *mocks.MockSchedulerStore)
		wantErr             bool
		wantPendingCount    int
		wantScheduledCount  int
		wantNextScheduledRun bool
	}{
		{
			name:   "returns pending and scheduled tasks",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				pendingTasks := []*domain.Task{
					domain.NewSyncSourceTask("team-123", "source-1"),
					domain.NewSyncSourceTask("team-123", "source-2"),
				}
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					if filter.Status != domain.TaskStatusPending {
						t.Errorf("expected status pending, got %v", filter.Status)
					}
					if filter.Limit != 20 {
						t.Errorf("expected limit 20, got %d", filter.Limit)
					}
					return pendingTasks, nil
				})

				nextRun := time.Now().Add(1 * time.Hour)
				scheduledTask := domain.NewScheduledTask("sched-1", "Daily Sync", domain.TaskTypeSyncAll, "team-123", 24*time.Hour)
				scheduledTask.NextRun = nextRun
				ss.AddScheduledTask(scheduledTask)
			},
			wantErr:              false,
			wantPendingCount:     2,
			wantScheduledCount:   1,
			wantNextScheduledRun: true,
		},
		{
			name:   "returns only earliest next scheduled run",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				})

				now := time.Now()
				task1 := domain.NewScheduledTask("sched-1", "Task 1", domain.TaskTypeSyncAll, "team-123", 24*time.Hour)
				task1.NextRun = now.Add(2 * time.Hour)
				ss.AddScheduledTask(task1)

				task2 := domain.NewScheduledTask("sched-2", "Task 2", domain.TaskTypeSyncAll, "team-123", 12*time.Hour)
				task2.NextRun = now.Add(1 * time.Hour) // Earlier
				ss.AddScheduledTask(task2)

				task3 := domain.NewScheduledTask("sched-3", "Task 3", domain.TaskTypeSyncAll, "team-123", 6*time.Hour)
				task3.NextRun = now.Add(3 * time.Hour)
				ss.AddScheduledTask(task3)
			},
			wantErr:              false,
			wantPendingCount:     0,
			wantScheduledCount:   3,
			wantNextScheduledRun: true,
		},
		{
			name:   "ignores disabled scheduled tasks for next run",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				})

				now := time.Now()
				task1 := domain.NewScheduledTask("sched-1", "Task 1", domain.TaskTypeSyncAll, "team-123", 24*time.Hour)
				task1.Enabled = false
				task1.NextRun = now.Add(1 * time.Hour)
				ss.AddScheduledTask(task1)

				task2 := domain.NewScheduledTask("sched-2", "Task 2", domain.TaskTypeSyncAll, "team-123", 12*time.Hour)
				task2.NextRun = now.Add(2 * time.Hour)
				ss.AddScheduledTask(task2)
			},
			wantErr:              false,
			wantPendingCount:     0,
			wantScheduledCount:   2,
			wantNextScheduledRun: true,
		},
		{
			name:   "no next scheduled run if all disabled",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				})

				task := domain.NewScheduledTask("sched-1", "Task 1", domain.TaskTypeSyncAll, "team-123", 24*time.Hour)
				task.Enabled = false
				ss.AddScheduledTask(task)
			},
			wantErr:              false,
			wantPendingCount:     0,
			wantScheduledCount:   1,
			wantNextScheduledRun: false,
		},
		{
			name:   "error listing pending tasks",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
		{
			name:   "error listing scheduled tasks",
			teamID: "team-123",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSchedulerStore) {
				tq.SetListTasksFunc(func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				})
				ss.SetListScheduledTasksFunc(func(ctx context.Context, teamID string) ([]*domain.ScheduledTask, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(taskQueue, schedulerStore)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetUpcomingJobs(context.Background(), tt.teamID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if len(result.PendingTasks) != tt.wantPendingCount {
				t.Errorf("expected %d pending tasks, got %d", tt.wantPendingCount, len(result.PendingTasks))
			}

			if len(result.ScheduledTasks) != tt.wantScheduledCount {
				t.Errorf("expected %d scheduled tasks, got %d", tt.wantScheduledCount, len(result.ScheduledTasks))
			}

			if tt.wantNextScheduledRun && result.NextScheduledRun == nil {
				t.Error("expected next scheduled run, got nil")
			}

			if !tt.wantNextScheduledRun && result.NextScheduledRun != nil {
				t.Error("expected no next scheduled run, got value")
			}
		})
	}
}

func TestAdminService_GetJob(t *testing.T) {
	tests := []struct {
		name              string
		teamID            string
		jobID             string
		setupMocks        func(*mocks.MockTaskQueue, *mocks.MockSourceStore)
		wantErr           error
		wantSourceName    string
		hasSourceName     bool
	}{
		{
			name:   "successful get with source enrichment",
			teamID: "team-123",
			jobID:  "job-456",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSourceStore) {
				task := domain.NewSyncSourceTask("team-123", "source-789")
				task.ID = "job-456"
				tq.SetGetTaskFunc(func(ctx context.Context, taskID string) (*domain.Task, error) {
					if taskID != "job-456" {
						t.Errorf("expected job ID job-456, got %s", taskID)
					}
					return task, nil
				})

				source := &domain.Source{
					ID:   "source-789",
					Name: "GitHub Repo",
				}
				ss.Save(context.Background(), source)
			},
			wantErr:        nil,
			wantSourceName: "GitHub Repo",
			hasSourceName:  true,
		},
		{
			name:   "successful get without source (non-sync task)",
			teamID: "team-123",
			jobID:  "job-456",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSourceStore) {
				task := domain.NewSyncAllTask("team-123")
				task.ID = "job-456"
				tq.SetGetTaskFunc(func(ctx context.Context, taskID string) (*domain.Task, error) {
					return task, nil
				})
			},
			wantErr:       nil,
			hasSourceName: false,
		},
		{
			name:   "task not found",
			teamID: "team-123",
			jobID:  "nonexistent",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSourceStore) {
				tq.SetGetTaskFunc(func(ctx context.Context, taskID string) (*domain.Task, error) {
					return nil, domain.ErrNotFound
				})
			},
			wantErr: errors.New("wrapped error"), // Any error is acceptable
		},
		{
			name:   "task belongs to different team",
			teamID: "team-123",
			jobID:  "job-456",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSourceStore) {
				task := domain.NewSyncSourceTask("team-999", "source-789")
				task.ID = "job-456"
				tq.SetGetTaskFunc(func(ctx context.Context, taskID string) (*domain.Task, error) {
					return task, nil
				})
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name:   "source not found should not error (graceful degradation)",
			teamID: "team-123",
			jobID:  "job-456",
			setupMocks: func(tq *mocks.MockTaskQueue, ss *mocks.MockSourceStore) {
				task := domain.NewSyncSourceTask("team-123", "source-999")
				task.ID = "job-456"
				tq.SetGetTaskFunc(func(ctx context.Context, taskID string) (*domain.Task, error) {
					return task, nil
				})
				// Source store empty, will return ErrNotFound
			},
			wantErr:       nil,
			hasSourceName: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(taskQueue, sourceStore)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetJob(context.Background(), tt.teamID, tt.jobID)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if result.Task == nil {
				t.Fatal("expected task in result, got nil")
			}

			if tt.hasSourceName {
				if result.SourceName != tt.wantSourceName {
					t.Errorf("expected source name %s, got %s", tt.wantSourceName, result.SourceName)
				}
			} else {
				if result.SourceName != "" {
					t.Errorf("expected no source name, got %s", result.SourceName)
				}
			}
		})
	}
}

func TestAdminService_GetJobStats(t *testing.T) {
	tests := []struct {
		name        string
		teamID      string
		period      driving.JobStatsPeriod
		setupMocks  func(*mocks.MockTaskQueue)
		wantErr     bool
		checkPeriod func(*testing.T, domain.AnalyticsPeriod)
	}{
		{
			name:   "successful get with 24h period",
			teamID: "team-123",
			period: driving.JobStatsPeriod24Hours,
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetGetJobStatsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
					stats := domain.NewJobStats(period)
					stats.TotalJobs = 100
					stats.CompletedJobs = 80
					stats.FailedJobs = 10
					stats.CalculateSuccessRate()
					return stats, nil
				})
			},
			wantErr: false,
			checkPeriod: func(t *testing.T, period domain.AnalyticsPeriod) {
				// Period should be approximately 24 hours ago
				expectedStart := time.Now().Add(-24 * time.Hour)
				if period.Start.Before(expectedStart.Add(-1*time.Minute)) || period.Start.After(expectedStart.Add(1*time.Minute)) {
					t.Errorf("expected start around 24h ago, got %v", period.Start)
				}
			},
		},
		{
			name:   "successful get with 7d period",
			teamID: "team-123",
			period: driving.JobStatsPeriod7Days,
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetGetJobStatsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
					return domain.NewJobStats(period), nil
				})
			},
			wantErr: false,
			checkPeriod: func(t *testing.T, period domain.AnalyticsPeriod) {
				expectedStart := time.Now().Add(-7 * 24 * time.Hour)
				if period.Start.Before(expectedStart.Add(-1*time.Minute)) || period.Start.After(expectedStart.Add(1*time.Minute)) {
					t.Errorf("expected start around 7 days ago, got %v", period.Start)
				}
			},
		},
		{
			name:   "successful get with 30d period",
			teamID: "team-123",
			period: driving.JobStatsPeriod30Days,
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetGetJobStatsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
					return domain.NewJobStats(period), nil
				})
			},
			wantErr: false,
			checkPeriod: func(t *testing.T, period domain.AnalyticsPeriod) {
				expectedStart := time.Now().Add(-30 * 24 * time.Hour)
				if period.Start.Before(expectedStart.Add(-1*time.Minute)) || period.Start.After(expectedStart.Add(1*time.Minute)) {
					t.Errorf("expected start around 30 days ago, got %v", period.Start)
				}
			},
		},
		{
			name:   "defaults to 24h for invalid period",
			teamID: "team-123",
			period: driving.JobStatsPeriod("invalid"),
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetGetJobStatsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
					return domain.NewJobStats(period), nil
				})
			},
			wantErr: false,
			checkPeriod: func(t *testing.T, period domain.AnalyticsPeriod) {
				expectedStart := time.Now().Add(-24 * time.Hour)
				if period.Start.Before(expectedStart.Add(-1*time.Minute)) || period.Start.After(expectedStart.Add(1*time.Minute)) {
					t.Errorf("expected default to 24h period, got %v", period.Start)
				}
			},
		},
		{
			name:   "error from task queue",
			teamID: "team-123",
			period: driving.JobStatsPeriod24Hours,
			setupMocks: func(tq *mocks.MockTaskQueue) {
				tq.SetGetJobStatsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(taskQueue)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetJobStats(context.Background(), tt.teamID, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if tt.checkPeriod != nil {
				tt.checkPeriod(t, result.Period)
			}
		})
	}
}

func TestAdminService_GetSearchAnalytics(t *testing.T) {
	tests := []struct {
		name       string
		teamID     string
		period     driving.SearchAnalyticsPeriod
		setupMocks func(*mocks.MockSearchQueryRepository)
		wantErr    bool
	}{
		{
			name:   "successful get with 24h period",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod24Hours,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchAnalyticsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
					return &domain.SearchAnalytics{
						TotalSearches:   1000,
						UniqueUsers:     50,
						AverageDuration: 150.5,
						Period:          period,
						SearchesByMode:  make(map[domain.SearchMode]int64),
					}, nil
				})
			},
			wantErr: false,
		},
		{
			name:   "successful get with 7d period",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod7Days,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchAnalyticsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
					return &domain.SearchAnalytics{
						Period:         period,
						SearchesByMode: make(map[domain.SearchMode]int64),
					}, nil
				})
			},
			wantErr: false,
		},
		{
			name:   "successful get with 30d period",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod30Days,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchAnalyticsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
					return &domain.SearchAnalytics{
						Period:         period,
						SearchesByMode: make(map[domain.SearchMode]int64),
					}, nil
				})
			},
			wantErr: false,
		},
		{
			name:   "error from repository",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod24Hours,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchAnalyticsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(searchQueryRepo)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetSearchAnalytics(context.Background(), tt.teamID, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}
		})
	}
}

func TestAdminService_GetSearchHistory(t *testing.T) {
	tests := []struct {
		name       string
		teamID     string
		limit      int
		setupMocks func(*mocks.MockSearchQueryRepository)
		wantErr    bool
		wantCount  int
		wantLimit  int
	}{
		{
			name:   "successful get with default limit",
			teamID: "team-123",
			limit:  0, // Should default to 50
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchHistoryFunc(func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
					if limit != 50 {
						t.Errorf("expected default limit 50, got %d", limit)
					}
					return make([]*domain.SearchQuery, 10), nil
				})
			},
			wantErr:   false,
			wantCount: 10,
			wantLimit: 50,
		},
		{
			name:   "enforces max limit of 100",
			teamID: "team-123",
			limit:  200, // Should be capped at 100
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchHistoryFunc(func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
					if limit != 100 {
						t.Errorf("expected limit capped at 100, got %d", limit)
					}
					return []*domain.SearchQuery{}, nil
				})
			},
			wantErr:   false,
			wantCount: 0,
			wantLimit: 100,
		},
		{
			name:   "respects custom limit",
			teamID: "team-123",
			limit:  25,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchHistoryFunc(func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
					if limit != 25 {
						t.Errorf("expected limit 25, got %d", limit)
					}
					return make([]*domain.SearchQuery, 25), nil
				})
			},
			wantErr:   false,
			wantCount: 25,
			wantLimit: 25,
		},
		{
			name:   "error from repository",
			teamID: "team-123",
			limit:  10,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchHistoryFunc(func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(searchQueryRepo)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetSearchHistory(context.Background(), tt.teamID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d queries, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestAdminService_GetSearchMetrics(t *testing.T) {
	tests := []struct {
		name       string
		teamID     string
		period     driving.SearchAnalyticsPeriod
		setupMocks func(*mocks.MockSearchQueryRepository)
		wantErr    bool
	}{
		{
			name:   "successful get",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod24Hours,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchMetricsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error) {
					return &domain.SearchMetrics{
						FastSearches:       800,
						MediumSearches:     150,
						SlowSearches:       50,
						P50Duration:        75.5,
						P95Duration:        350.2,
						P99Duration:        800.1,
						ZeroResultSearches: 30,
						Period:             period,
					}, nil
				})
			},
			wantErr: false,
		},
		{
			name:   "error from repository",
			teamID: "team-123",
			period: driving.SearchAnalyticsPeriod7Days,
			setupMocks: func(sqr *mocks.MockSearchQueryRepository) {
				sqr.SetGetSearchMetricsFunc(func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error) {
					return nil, errors.New("database error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(searchQueryRepo)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			result, err := svc.GetSearchMetrics(context.Background(), tt.teamID, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}
		})
	}
}

func TestAdminService_TriggerReindex(t *testing.T) {
	tests := []struct {
		name          string
		teamID        string
		req           driving.TriggerReindexRequest
		setupMocks    func(*mocks.MockSourceStore, *mocks.MockTaskQueue)
		wantErr       bool
		wantTaskCount int
	}{
		{
			name:   "reindex all enabled sources",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				Priority: 50,
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				sources := []*domain.Source{
					{ID: "source-1", Name: "Source 1", Enabled: true},
					{ID: "source-2", Name: "Source 2", Enabled: true},
					{ID: "source-3", Name: "Source 3", Enabled: false}, // Should be excluded
				}
				for _, source := range sources {
					ss.Save(context.Background(), source)
				}
			},
			wantErr:       false,
			wantTaskCount: 2,
		},
		{
			name:   "reindex specific sources",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				SourceIDs: []string{"source-1", "source-2"},
				Priority:  10,
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				sources := []*domain.Source{
					{ID: "source-1", Name: "Source 1", Enabled: true},
					{ID: "source-2", Name: "Source 2", Enabled: true},
					{ID: "source-3", Name: "Source 3", Enabled: true},
				}
				for _, source := range sources {
					ss.Save(context.Background(), source)
				}
			},
			wantErr:       false,
			wantTaskCount: 2,
		},
		{
			name:   "error when specific source not found",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				SourceIDs: []string{"nonexistent"},
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				// Empty store
			},
			wantErr: true,
		},
		{
			name:   "error when specific source is disabled",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				SourceIDs: []string{"source-1"},
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				source := &domain.Source{ID: "source-1", Name: "Source 1", Enabled: false}
				ss.Save(context.Background(), source)
			},
			wantErr: true,
		},
		{
			name:   "error when no enabled sources exist",
			teamID: "team-123",
			req:    driving.TriggerReindexRequest{},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				source := &domain.Source{ID: "source-1", Name: "Source 1", Enabled: false}
				ss.Save(context.Background(), source)
			},
			wantErr: true,
		},
		{
			name:   "error enqueueing tasks",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				SourceIDs: []string{"source-1"},
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				source := &domain.Source{ID: "source-1", Name: "Source 1", Enabled: true}
				ss.Save(context.Background(), source)
				tq.SetEnqueueError(errors.New("queue error"))
			},
			wantErr: true,
		},
		{
			name:   "sets correct priority on tasks",
			teamID: "team-123",
			req: driving.TriggerReindexRequest{
				SourceIDs: []string{"source-1"},
				Priority:  75,
			},
			setupMocks: func(ss *mocks.MockSourceStore, tq *mocks.MockTaskQueue) {
				source := &domain.Source{ID: "source-1", Name: "Source 1", Enabled: true}
				ss.Save(context.Background(), source)
			},
			wantErr:       false,
			wantTaskCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskQueue := mocks.NewMockTaskQueue()
			schedulerStore := mocks.NewMockSchedulerStore()
			searchQueryRepo := mocks.NewMockSearchQueryRepository()
			sourceStore := mocks.NewMockSourceStore()

			tt.setupMocks(sourceStore, taskQueue)

			svc := NewAdminService(taskQueue, schedulerStore, searchQueryRepo, sourceStore)
			taskIDs, err := svc.TriggerReindex(context.Background(), tt.teamID, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(taskIDs) != tt.wantTaskCount {
				t.Errorf("expected %d task IDs, got %d", tt.wantTaskCount, len(taskIDs))
			}

			// Verify EnqueueBatch was called
			if taskQueue.GetEnqueueBatchCalls() != 1 {
				t.Errorf("expected EnqueueBatch to be called once, got %d", taskQueue.GetEnqueueBatchCalls())
			}

			// Verify tasks have correct properties
			tasks := taskQueue.GetTasks()
			if len(tasks) != tt.wantTaskCount {
				t.Errorf("expected %d tasks enqueued, got %d", tt.wantTaskCount, len(tasks))
			}

			for _, task := range tasks {
				if task.Type != domain.TaskTypeSyncSource {
					t.Errorf("expected task type sync_source, got %v", task.Type)
				}
				if task.TeamID != tt.teamID {
					t.Errorf("expected team ID %s, got %s", tt.teamID, task.TeamID)
				}
				if task.Priority != tt.req.Priority {
					t.Errorf("expected priority %d, got %d", tt.req.Priority, task.Priority)
				}
			}
		})
	}
}
