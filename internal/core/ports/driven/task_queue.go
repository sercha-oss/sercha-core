package driven

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// TaskQueue handles background task queuing and processing.
// Implementations can use Redis (preferred) or Postgres (fallback).
type TaskQueue interface {
	// Enqueue adds a task to the queue for processing.
	// The task will be picked up by a worker based on priority and scheduled time.
	Enqueue(ctx context.Context, task *domain.Task) error

	// EnqueueBatch adds multiple tasks to the queue atomically.
	// If any task fails to enqueue, all tasks are rolled back.
	EnqueueBatch(ctx context.Context, tasks []*domain.Task) error

	// Dequeue retrieves the next available task for processing.
	// This should block until a task is available or context is cancelled.
	// The task is marked as processing and will not be returned to other workers.
	// Returns nil, nil if no tasks are available (for non-blocking implementations).
	Dequeue(ctx context.Context) (*domain.Task, error)

	// DequeueWithTimeout retrieves the next available task, waiting up to timeout.
	// Returns nil, nil if timeout is reached with no tasks available.
	DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error)

	// Ack acknowledges successful completion of a task.
	// The task is removed from the queue.
	Ack(ctx context.Context, taskID string) error

	// Nack indicates task processing failed and should be retried.
	// The task is returned to the queue with updated retry count.
	// If max retries exceeded, task is moved to failed state.
	Nack(ctx context.Context, taskID string, reason string) error

	// GetTask retrieves a task by ID (for status checking).
	GetTask(ctx context.Context, taskID string) (*domain.Task, error)

	// ListTasks retrieves tasks matching the filter criteria.
	ListTasks(ctx context.Context, filter TaskFilter) ([]*domain.Task, error)

	// CancelTask marks a pending task as cancelled.
	// Returns error if task is already processing or completed.
	CancelTask(ctx context.Context, taskID string) error

	// PurgeTasks removes completed/failed tasks older than the specified age.
	// This is used for cleanup.
	PurgeTasks(ctx context.Context, olderThan int) (int, error)

	// Stats returns queue statistics.
	Stats(ctx context.Context) (*QueueStats, error)

	// Ping checks if the queue backend is healthy.
	Ping(ctx context.Context) error

	// GetJobStats computes aggregated job statistics for a time period
	// This is used by the admin dashboard to show job execution metrics
	GetJobStats(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error)

	// CountTasks returns the total number of tasks matching the filter
	// This is used for pagination to determine if there are more results
	CountTasks(ctx context.Context, filter TaskFilter) (int64, error)

	// Close cleans up resources.
	Close() error
}

// TaskFilter specifies criteria for listing tasks
type TaskFilter struct {
	// TeamID filters by team (required)
	TeamID string

	// Status filters by task status (optional, empty means all)
	Status domain.TaskStatus

	// Type filters by task type (optional, empty means all)
	Type domain.TaskType

	// Limit is the maximum number of tasks to return
	Limit int

	// Offset is the number of tasks to skip (for pagination)
	Offset int
}

// QueueStats contains queue statistics
type QueueStats struct {
	// PendingCount is the number of tasks waiting to be processed
	PendingCount int64 `json:"pending_count"`

	// ProcessingCount is the number of tasks currently being processed
	ProcessingCount int64 `json:"processing_count"`

	// CompletedCount is the number of successfully completed tasks
	CompletedCount int64 `json:"completed_count"`

	// FailedCount is the number of tasks that failed after all retries
	FailedCount int64 `json:"failed_count"`

	// OldestPendingAge is the age of the oldest pending task in seconds
	OldestPendingAge int64 `json:"oldest_pending_age"`
}

// SchedulerStore handles persistence for scheduled tasks.
// This is separate from TaskQueue because scheduled tasks are configuration,
// not transient queue items.
type SchedulerStore interface {
	// GetScheduledTask retrieves a scheduled task by ID
	GetScheduledTask(ctx context.Context, id string) (*domain.ScheduledTask, error)

	// ListScheduledTasks retrieves all scheduled tasks for a team
	ListScheduledTasks(ctx context.Context, teamID string) ([]*domain.ScheduledTask, error)

	// SaveScheduledTask creates or updates a scheduled task
	SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error

	// DeleteScheduledTask removes a scheduled task
	DeleteScheduledTask(ctx context.Context, id string) error

	// GetDueScheduledTasks retrieves scheduled tasks that are due to run
	GetDueScheduledTasks(ctx context.Context) ([]*domain.ScheduledTask, error)

	// UpdateLastRun updates the last run time and next run time
	UpdateLastRun(ctx context.Context, id string, lastError string) error
}
