package domain

import (
	"testing"
	"time"
)

func TestNewJobStats(t *testing.T) {
	period := Last24Hours()
	stats := NewJobStats(period)

	if stats == nil {
		t.Fatal("expected non-nil JobStats")
	}
	if stats.JobsByType == nil {
		t.Error("expected JobsByType map to be initialized")
	}
	if !stats.Period.Start.Equal(period.Start) {
		t.Error("Period.Start not set correctly")
	}
	if !stats.Period.End.Equal(period.End) {
		t.Error("Period.End not set correctly")
	}
}

func TestJobStats_CalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name          string
		completed     int64
		failed        int64
		expectedRate  float64
		description   string
	}{
		{
			name:         "all successful",
			completed:    100,
			failed:       0,
			expectedRate: 100.0,
			description:  "100% success rate when no failures",
		},
		{
			name:         "all failed",
			completed:    0,
			failed:       100,
			expectedRate: 0.0,
			description:  "0% success rate when all failed",
		},
		{
			name:         "half successful",
			completed:    50,
			failed:       50,
			expectedRate: 50.0,
			description:  "50% success rate with equal success/failure",
		},
		{
			name:         "75% successful",
			completed:    75,
			failed:       25,
			expectedRate: 75.0,
			description:  "75% success rate",
		},
		{
			name:         "no jobs",
			completed:    0,
			failed:       0,
			expectedRate: 0.0,
			description:  "0% success rate when no jobs completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := NewJobStats(Last24Hours())
			stats.CompletedJobs = tt.completed
			stats.FailedJobs = tt.failed

			stats.CalculateSuccessRate()

			if stats.SuccessRate != tt.expectedRate {
				t.Errorf("%s: expected success rate %.2f, got %.2f",
					tt.description, tt.expectedRate, stats.SuccessRate)
			}
		})
	}
}

func TestNewJobHistory(t *testing.T) {
	tasks := []*Task{
		NewTask(TaskTypeSyncSource, "team-1", nil),
		NewTask(TaskTypeSyncAll, "team-1", nil),
	}
	totalCount := int64(10)
	limit := 5

	history := NewJobHistory(tasks, totalCount, limit)

	if history == nil {
		t.Fatal("expected non-nil JobHistory")
	}
	if len(history.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(history.Jobs))
	}
	if history.TotalCount != totalCount {
		t.Errorf("expected total count %d, got %d", totalCount, history.TotalCount)
	}
	if !history.HasMore {
		t.Error("expected HasMore to be true when jobs < totalCount")
	}
}

func TestJobHistory_HasMore(t *testing.T) {
	tests := []struct {
		name        string
		jobCount    int
		totalCount  int64
		limit       int
		expectMore  bool
	}{
		{
			name:       "has more pages",
			jobCount:   5,
			totalCount: 10,
			limit:      5,
			expectMore: true,
		},
		{
			name:       "last page",
			jobCount:   5,
			totalCount: 5,
			limit:      5,
			expectMore: false,
		},
		{
			name:       "empty results",
			jobCount:   0,
			totalCount: 0,
			limit:      10,
			expectMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := make([]*Task, tt.jobCount)
			for i := 0; i < tt.jobCount; i++ {
				tasks[i] = NewTask(TaskTypeSyncSource, "team-1", nil)
			}

			history := NewJobHistory(tasks, tt.totalCount, tt.limit)

			if history.HasMore != tt.expectMore {
				t.Errorf("expected HasMore=%v, got %v", tt.expectMore, history.HasMore)
			}
		})
	}
}

func TestNewUpcomingJobs(t *testing.T) {
	now := time.Now()
	future1 := now.Add(1 * time.Hour)
	future2 := now.Add(2 * time.Hour)

	pendingTasks := []*Task{
		NewTask(TaskTypeSyncSource, "team-1", nil),
		NewTask(TaskTypeSyncAll, "team-1", nil),
	}

	scheduledTasks := []*ScheduledTask{
		{
			ID:       "sched-1",
			Name:     "Task 1",
			Type:     TaskTypeSyncAll,
			TeamID:   "team-1",
			Interval: time.Hour,
			Enabled:  true,
			NextRun:  future2,
		},
		{
			ID:       "sched-2",
			Name:     "Task 2",
			Type:     TaskTypeSyncSource,
			TeamID:   "team-1",
			Interval: time.Hour,
			Enabled:  true,
			NextRun:  future1, // Earlier than future2
		},
	}

	upcoming := NewUpcomingJobs(pendingTasks, scheduledTasks)

	if upcoming == nil {
		t.Fatal("expected non-nil UpcomingJobs")
	}
	if len(upcoming.PendingTasks) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(upcoming.PendingTasks))
	}
	if len(upcoming.ScheduledTasks) != 2 {
		t.Errorf("expected 2 scheduled tasks, got %d", len(upcoming.ScheduledTasks))
	}
	if upcoming.NextScheduledRun == nil {
		t.Error("expected NextScheduledRun to be set")
	} else if !upcoming.NextScheduledRun.Equal(future1) {
		t.Errorf("expected NextScheduledRun to be %v, got %v", future1, *upcoming.NextScheduledRun)
	}
}

func TestUpcomingJobs_NextScheduledRun_DisabledTasks(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)

	scheduledTasks := []*ScheduledTask{
		{
			ID:       "sched-1",
			Name:     "Disabled Task",
			Type:     TaskTypeSyncAll,
			TeamID:   "team-1",
			Interval: time.Hour,
			Enabled:  false, // Disabled
			NextRun:  future,
		},
	}

	upcoming := NewUpcomingJobs(nil, scheduledTasks)

	if upcoming.NextScheduledRun != nil {
		t.Error("expected NextScheduledRun to be nil when all tasks are disabled")
	}
}

func TestUpcomingJobs_NextScheduledRun_NoScheduledTasks(t *testing.T) {
	pendingTasks := []*Task{NewTask(TaskTypeSyncSource, "team-1", nil)}

	upcoming := NewUpcomingJobs(pendingTasks, nil)

	if upcoming.NextScheduledRun != nil {
		t.Error("expected NextScheduledRun to be nil when no scheduled tasks")
	}
}

func TestNewJobDetail(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-1", map[string]string{"source_id": "src-123"})

	detail := NewJobDetail(task)

	if detail == nil {
		t.Fatal("expected non-nil JobDetail")
	}
	if detail.Task != task {
		t.Error("Task not set correctly")
	}
	if detail.SourceName != "" {
		t.Error("expected empty SourceName initially")
	}
	if len(detail.ExecutionLogs) != 0 {
		t.Error("expected empty ExecutionLogs initially")
	}
	if len(detail.RetryHistory) != 0 {
		t.Error("expected empty RetryHistory initially")
	}
}

func TestJobDetail_WithSourceName(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-1", nil)
	sourceName := "Test Source"

	detail := NewJobDetail(task).WithSourceName(sourceName)

	if detail.SourceName != sourceName {
		t.Errorf("expected source name %s, got %s", sourceName, detail.SourceName)
	}
}

func TestJobDetail_WithExecutionLogs(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-1", nil)
	logs := []string{"Started processing", "Completed successfully"}

	detail := NewJobDetail(task).WithExecutionLogs(logs)

	if len(detail.ExecutionLogs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(detail.ExecutionLogs))
	}
	if detail.ExecutionLogs[0] != "Started processing" {
		t.Error("first log not set correctly")
	}
}

func TestJobDetail_WithRetryHistory(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-1", nil)
	history := []*RetryAttempt{
		{
			Attempt:   1,
			Error:     "connection timeout",
			Timestamp: time.Now().Add(-1 * time.Hour),
		},
		{
			Attempt:   2,
			Error:     "rate limit exceeded",
			Timestamp: time.Now().Add(-30 * time.Minute),
		},
	}

	detail := NewJobDetail(task).WithRetryHistory(history)

	if len(detail.RetryHistory) != 2 {
		t.Errorf("expected 2 retry attempts, got %d", len(detail.RetryHistory))
	}
	if detail.RetryHistory[0].Attempt != 1 {
		t.Error("first retry attempt not set correctly")
	}
	if detail.RetryHistory[1].Error != "rate limit exceeded" {
		t.Error("second retry error not set correctly")
	}
}

func TestJobDetail_ChainedBuilders(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-1", nil)

	detail := NewJobDetail(task).
		WithSourceName("Test Source").
		WithExecutionLogs([]string{"log1", "log2"}).
		WithRetryHistory([]*RetryAttempt{
			{Attempt: 1, Error: "error1", Timestamp: time.Now()},
		})

	if detail.SourceName != "Test Source" {
		t.Error("SourceName not set in chained call")
	}
	if len(detail.ExecutionLogs) != 2 {
		t.Error("ExecutionLogs not set in chained call")
	}
	if len(detail.RetryHistory) != 1 {
		t.Error("RetryHistory not set in chained call")
	}
}

func TestJobStats_Structure(t *testing.T) {
	stats := &JobStats{
		TotalJobs:       1000,
		PendingJobs:     50,
		ProcessingJobs:  10,
		CompletedJobs:   800,
		FailedJobs:      140,
		SuccessRate:     85.1,
		AverageDuration: 2500.0,
		TotalRetries:    45,
		JobsByType: map[TaskType]int64{
			TaskTypeSyncSource: 700,
			TaskTypeSyncAll:    300,
		},
		Period: Last24Hours(),
	}

	if stats.TotalJobs != 1000 {
		t.Error("TotalJobs not set correctly")
	}
	if stats.CompletedJobs != 800 {
		t.Error("CompletedJobs not set correctly")
	}
	if len(stats.JobsByType) != 2 {
		t.Error("JobsByType not set correctly")
	}
}
