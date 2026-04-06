package domain

import "time"

// JobStats represents aggregated job statistics
// This is a value object (no identity) representing computed statistics
type JobStats struct {
	// Total jobs across all statuses
	TotalJobs int64 `json:"total_jobs"`

	// PendingJobs is the number of jobs waiting to be processed
	PendingJobs int64 `json:"pending_jobs"`

	// ProcessingJobs is the number of jobs currently being processed
	ProcessingJobs int64 `json:"processing_jobs"`

	// CompletedJobs is the number of successfully completed jobs
	CompletedJobs int64 `json:"completed_jobs"`

	// FailedJobs is the number of failed jobs
	FailedJobs int64 `json:"failed_jobs"`

	// SuccessRate is the percentage of successful jobs (0-100)
	SuccessRate float64 `json:"success_rate"`

	// AverageDuration is the average job duration in milliseconds
	AverageDuration float64 `json:"average_duration_ms"`

	// TotalRetries is the total number of retry attempts
	TotalRetries int64 `json:"total_retries"`

	// JobsByType shows the breakdown by task type
	JobsByType map[TaskType]int64 `json:"jobs_by_type"`

	// Period is the time range for these statistics
	Period AnalyticsPeriod `json:"period"`
}

// CalculateSuccessRate computes the success rate percentage
func (js *JobStats) CalculateSuccessRate() {
	completed := js.CompletedJobs + js.FailedJobs
	if completed > 0 {
		js.SuccessRate = (float64(js.CompletedJobs) / float64(completed)) * 100
	} else {
		js.SuccessRate = 0
	}
}

// NewJobStats creates a new job statistics value object
func NewJobStats(period AnalyticsPeriod) *JobStats {
	return &JobStats{
		JobsByType: make(map[TaskType]int64),
		Period:     period,
	}
}

// JobHistory represents a summarized view of job execution history
// This is a value object used for the job history endpoint
type JobHistory struct {
	// Jobs is the list of historical job executions
	Jobs []*Task `json:"jobs"`

	// TotalCount is the total number of jobs matching the query
	TotalCount int64 `json:"total_count"`

	// HasMore indicates if there are more jobs beyond the current page
	HasMore bool `json:"has_more"`
}

// NewJobHistory creates a new job history value object
func NewJobHistory(jobs []*Task, totalCount int64, limit int) *JobHistory {
	hasMore := int64(len(jobs)) < totalCount
	return &JobHistory{
		Jobs:       jobs,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}
}

// UpcomingJobs represents pending and scheduled jobs
// This is a value object combining different job types for the upcoming jobs view
type UpcomingJobs struct {
	// PendingTasks are tasks ready to be processed now
	PendingTasks []*Task `json:"pending_tasks"`

	// ScheduledTasks are recurring task schedules
	ScheduledTasks []*ScheduledTask `json:"scheduled_tasks"`

	// NextScheduledRun is when the next scheduled task will run
	NextScheduledRun *time.Time `json:"next_scheduled_run,omitempty"`
}

// NewUpcomingJobs creates a new upcoming jobs value object
func NewUpcomingJobs(pendingTasks []*Task, scheduledTasks []*ScheduledTask) *UpcomingJobs {
	uj := &UpcomingJobs{
		PendingTasks:   pendingTasks,
		ScheduledTasks: scheduledTasks,
	}

	// Find the earliest next scheduled run
	for _, st := range scheduledTasks {
		if st.Enabled {
			if uj.NextScheduledRun == nil || st.NextRun.Before(*uj.NextScheduledRun) {
				next := st.NextRun
				uj.NextScheduledRun = &next
			}
		}
	}

	return uj
}

// JobDetail represents detailed information about a specific job
// This is a value object that combines task information with execution context
type JobDetail struct {
	// Task is the main task information
	Task *Task `json:"task"`

	// SourceName is the name of the source (if applicable)
	SourceName string `json:"source_name,omitempty"`

	// ExecutionLogs contains log entries for this job (future enhancement)
	ExecutionLogs []string `json:"execution_logs,omitempty"`

	// RetryHistory shows previous retry attempts
	RetryHistory []*RetryAttempt `json:"retry_history,omitempty"`
}

// RetryAttempt represents a single retry attempt for a job
type RetryAttempt struct {
	// Attempt is the attempt number (1-indexed)
	Attempt int `json:"attempt"`

	// Error is the error message from this attempt
	Error string `json:"error"`

	// Timestamp is when this attempt occurred
	Timestamp time.Time `json:"timestamp"`
}

// NewJobDetail creates a new job detail value object
func NewJobDetail(task *Task) *JobDetail {
	return &JobDetail{
		Task: task,
	}
}

// WithSourceName adds the source name to the job detail
func (jd *JobDetail) WithSourceName(sourceName string) *JobDetail {
	jd.SourceName = sourceName
	return jd
}

// WithExecutionLogs adds execution logs to the job detail
func (jd *JobDetail) WithExecutionLogs(logs []string) *JobDetail {
	jd.ExecutionLogs = logs
	return jd
}

// WithRetryHistory adds retry history to the job detail
func (jd *JobDetail) WithRetryHistory(history []*RetryAttempt) *JobDetail {
	jd.RetryHistory = history
	return jd
}
