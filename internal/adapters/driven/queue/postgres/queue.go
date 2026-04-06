package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure Queue implements TaskQueue
var _ driven.TaskQueue = (*Queue)(nil)

// Queue implements TaskQueue using PostgreSQL with SKIP LOCKED for reliable task processing.
// This is the fallback queue when Redis is not available.
type Queue struct {
	db *sql.DB
}

// NewQueue creates a new PostgreSQL-backed task queue.
// Assumes the tasks table has been created via migrations.
func NewQueue(db *sql.DB) *Queue {
	return &Queue{db: db}
}

// Enqueue adds a task to the queue
func (q *Queue) Enqueue(ctx context.Context, task *domain.Task) error {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	query := `
		INSERT INTO tasks (
			id, type, team_id, payload, status, priority,
			attempts, max_attempts, error, created_at, updated_at, scheduled_for
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = q.db.ExecContext(ctx, query,
		task.ID,
		task.Type,
		task.TeamID,
		payload,
		task.Status,
		task.Priority,
		task.Attempts,
		task.MaxAttempts,
		task.Error,
		task.CreatedAt,
		task.UpdatedAt,
		task.ScheduledFor,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	return nil
}

// EnqueueBatch adds multiple tasks atomically
func (q *Queue) EnqueueBatch(ctx context.Context, tasks []*domain.Task) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		INSERT INTO tasks (
			id, type, team_id, payload, status, priority,
			attempts, max_attempts, error, created_at, updated_at, scheduled_for
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, task := range tasks {
		payload, err := json.Marshal(task.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload for task %s: %w", task.ID, err)
		}

		_, err = stmt.ExecContext(ctx,
			task.ID,
			task.Type,
			task.TeamID,
			payload,
			task.Status,
			task.Priority,
			task.Attempts,
			task.MaxAttempts,
			task.Error,
			task.CreatedAt,
			task.UpdatedAt,
			task.ScheduledFor,
		)
		if err != nil {
			return fmt.Errorf("insert task %s: %w", task.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Dequeue retrieves the next available task using SELECT FOR UPDATE SKIP LOCKED.
// This ensures only one worker gets each task even with multiple workers.
func (q *Queue) Dequeue(ctx context.Context) (*domain.Task, error) {
	return q.dequeue(ctx, 0)
}

// DequeueWithTimeout retrieves the next task, waiting up to timeout seconds
func (q *Queue) DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error) {
	return q.dequeue(ctx, timeout)
}

func (q *Queue) dequeue(ctx context.Context, timeoutSeconds int) (*domain.Task, error) {
	// Use a transaction to atomically select and update
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Select next available task with SKIP LOCKED to avoid contention
	selectQuery := `
		SELECT id, type, team_id, payload, status, priority,
			   attempts, max_attempts, error, created_at, updated_at,
			   started_at, completed_at, scheduled_for
		FROM tasks
		WHERE status = $1
		  AND scheduled_for <= NOW()
		ORDER BY priority DESC, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var task domain.Task
	var payload []byte
	var startedAt, completedAt sql.NullTime

	err = tx.QueryRowContext(ctx, selectQuery, domain.TaskStatusPending).Scan(
		&task.ID,
		&task.Type,
		&task.TeamID,
		&payload,
		&task.Status,
		&task.Priority,
		&task.Attempts,
		&task.MaxAttempts,
		&task.Error,
		&task.CreatedAt,
		&task.UpdatedAt,
		&startedAt,
		&completedAt,
		&task.ScheduledFor,
	)

	if err == sql.ErrNoRows {
		// No tasks available
		_ = tx.Rollback()

		// If timeout specified, wait and retry
		if timeoutSeconds > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(timeoutSeconds) * time.Second):
				// Retry after timeout
				return q.dequeue(ctx, 0)
			}
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select task: %w", err)
	}

	// Parse payload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &task.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	// Handle nullable times
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	// Mark task as processing
	now := time.Now()
	updateQuery := `
		UPDATE tasks
		SET status = $1, started_at = $2, updated_at = $3, attempts = attempts + 1
		WHERE id = $4
	`
	_, err = tx.ExecContext(ctx, updateQuery,
		domain.TaskStatusProcessing,
		now,
		now,
		task.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Update in-memory task state
	task.Status = domain.TaskStatusProcessing
	task.StartedAt = &now
	task.UpdatedAt = now
	task.Attempts++

	return &task, nil
}

// Ack marks a task as completed
func (q *Queue) Ack(ctx context.Context, taskID string) error {
	now := time.Now()
	query := `
		UPDATE tasks
		SET status = $1, completed_at = $2, updated_at = $3, error = ''
		WHERE id = $4
	`

	result, err := q.db.ExecContext(ctx, query,
		domain.TaskStatusCompleted,
		now,
		now,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Nack marks a task as failed, potentially scheduling a retry
func (q *Queue) Nack(ctx context.Context, taskID string, reason string) error {
	// First get the task to check retry count
	task, err := q.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	now := time.Now()

	if task.CanRetry() {
		// Schedule retry with exponential backoff
		backoff := time.Duration(1<<task.Attempts) * time.Second
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}

		query := `
			UPDATE tasks
			SET status = $1, error = $2, updated_at = $3, scheduled_for = $4
			WHERE id = $5
		`
		_, err = q.db.ExecContext(ctx, query,
			domain.TaskStatusPending,
			reason,
			now,
			now.Add(backoff),
			taskID,
		)
	} else {
		// Max retries exceeded, mark as failed
		query := `
			UPDATE tasks
			SET status = $1, error = $2, updated_at = $3
			WHERE id = $4
		`
		_, err = q.db.ExecContext(ctx, query,
			domain.TaskStatusFailed,
			reason,
			now,
			taskID,
		)
	}

	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	return nil
}

// GetTask retrieves a task by ID
func (q *Queue) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	query := `
		SELECT id, type, team_id, payload, status, priority,
			   attempts, max_attempts, error, created_at, updated_at,
			   started_at, completed_at, scheduled_for
		FROM tasks
		WHERE id = $1
	`

	var task domain.Task
	var payload []byte
	var startedAt, completedAt sql.NullTime

	err := q.db.QueryRowContext(ctx, query, taskID).Scan(
		&task.ID,
		&task.Type,
		&task.TeamID,
		&payload,
		&task.Status,
		&task.Priority,
		&task.Attempts,
		&task.MaxAttempts,
		&task.Error,
		&task.CreatedAt,
		&task.UpdatedAt,
		&startedAt,
		&completedAt,
		&task.ScheduledFor,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &task.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	return &task, nil
}

// ListTasks retrieves tasks matching the filter
func (q *Queue) ListTasks(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
	query := `
		SELECT id, type, team_id, payload, status, priority,
			   attempts, max_attempts, error, created_at, updated_at,
			   started_at, completed_at, scheduled_for
		FROM tasks
		WHERE team_id = $1
	`
	args := []any{filter.TeamID}
	argIndex := 2

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, filter.Type)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		var task domain.Task
		var payload []byte
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Type,
			&task.TeamID,
			&payload,
			&task.Status,
			&task.Priority,
			&task.Attempts,
			&task.MaxAttempts,
			&task.Error,
			&task.CreatedAt,
			&task.UpdatedAt,
			&startedAt,
			&completedAt,
			&task.ScheduledFor,
		)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &task.Payload); err != nil {
				return nil, fmt.Errorf("unmarshal payload: %w", err)
			}
		}

		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

// CancelTask cancels a pending task
func (q *Queue) CancelTask(ctx context.Context, taskID string) error {
	query := `
		UPDATE tasks
		SET status = $1, updated_at = $2, error = 'cancelled'
		WHERE id = $3 AND status = $4
	`

	result, err := q.db.ExecContext(ctx, query,
		domain.TaskStatusFailed,
		time.Now(),
		taskID,
		domain.TaskStatusPending,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found or not pending")
	}

	return nil
}

// PurgeTasks removes old completed/failed tasks
func (q *Queue) PurgeTasks(ctx context.Context, olderThanSeconds int) (int, error) {
	cutoff := time.Now().Add(-time.Duration(olderThanSeconds) * time.Second)

	query := `
		DELETE FROM tasks
		WHERE status IN ($1, $2)
		  AND updated_at < $3
	`

	result, err := q.db.ExecContext(ctx, query,
		domain.TaskStatusCompleted,
		domain.TaskStatusFailed,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("delete tasks: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return int(rows), nil
}

// Stats returns queue statistics
func (q *Queue) Stats(ctx context.Context) (*driven.QueueStats, error) {
	stats := &driven.QueueStats{}

	// Count by status
	query := `
		SELECT status, COUNT(*) FROM tasks GROUP BY status
	`
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}

		switch domain.TaskStatus(status) {
		case domain.TaskStatusPending:
			stats.PendingCount = count
		case domain.TaskStatusProcessing:
			stats.ProcessingCount = count
		case domain.TaskStatusCompleted:
			stats.CompletedCount = count
		case domain.TaskStatusFailed:
			stats.FailedCount = count
		}
	}

	// Get oldest pending task age
	ageQuery := `
		SELECT EXTRACT(EPOCH FROM (NOW() - MIN(created_at)))::bigint
		FROM tasks
		WHERE status = $1
	`
	var age sql.NullInt64
	err = q.db.QueryRowContext(ctx, ageQuery, domain.TaskStatusPending).Scan(&age)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query oldest age: %w", err)
	}
	if age.Valid {
		stats.OldestPendingAge = age.Int64
	}

	return stats, nil
}

// Ping checks database connectivity
func (q *Queue) Ping(ctx context.Context) error {
	return q.db.PingContext(ctx)
}

// GetJobStats computes aggregated job statistics for a time period
func (q *Queue) GetJobStats(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
	stats := domain.NewJobStats(period)

	// Get counts by status
	statusQuery := `
		SELECT
			COUNT(*) as total_jobs,
			COUNT(*) FILTER (WHERE status = $4) as pending_jobs,
			COUNT(*) FILTER (WHERE status = $5) as processing_jobs,
			COUNT(*) FILTER (WHERE status = $6) as completed_jobs,
			COUNT(*) FILTER (WHERE status = $7) as failed_jobs,
			COALESCE(SUM(attempts - 1), 0) as total_retries
		FROM tasks
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	err := q.db.QueryRowContext(ctx, statusQuery,
		teamID,
		period.Start,
		period.End,
		domain.TaskStatusPending,
		domain.TaskStatusProcessing,
		domain.TaskStatusCompleted,
		domain.TaskStatusFailed,
	).Scan(
		&stats.TotalJobs,
		&stats.PendingJobs,
		&stats.ProcessingJobs,
		&stats.CompletedJobs,
		&stats.FailedJobs,
		&stats.TotalRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("query job stats: %w", err)
	}

	// Calculate success rate
	stats.CalculateSuccessRate()

	// Get average duration for completed jobs
	durationQuery := `
		SELECT AVG(EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000)
		FROM tasks
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		  AND status = $4
		  AND started_at IS NOT NULL
		  AND completed_at IS NOT NULL
	`

	var avgDuration sql.NullFloat64
	err = q.db.QueryRowContext(ctx, durationQuery,
		teamID,
		period.Start,
		period.End,
		domain.TaskStatusCompleted,
	).Scan(&avgDuration)
	if err != nil {
		return nil, fmt.Errorf("query average duration: %w", err)
	}

	if avgDuration.Valid {
		stats.AverageDuration = avgDuration.Float64
	}

	// Get jobs by type
	typeQuery := `
		SELECT type, COUNT(*)
		FROM tasks
		WHERE team_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY type
	`

	rows, err := q.db.QueryContext(ctx, typeQuery, teamID, period.Start, period.End)
	if err != nil {
		return nil, fmt.Errorf("query jobs by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskType string
		var count int64
		if err := rows.Scan(&taskType, &count); err != nil {
			return nil, fmt.Errorf("scan job type: %w", err)
		}
		stats.JobsByType[domain.TaskType(taskType)] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job types: %w", err)
	}

	return stats, nil
}

// CountTasks returns the total number of tasks matching the filter
func (q *Queue) CountTasks(ctx context.Context, filter driven.TaskFilter) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE team_id = $1`
	args := []any{filter.TeamID}
	argIndex := 2

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, filter.Type)
	}

	var count int64
	err := q.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}

	return count, nil
}

// Close is a no-op for the Postgres queue (db connection managed externally)
func (q *Queue) Close() error {
	return nil
}

// SQL for creating the tasks table (to be used in migrations)
const CreateTasksTableSQL = `
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(36) PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    team_id VARCHAR(36) NOT NULL,
    payload JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    error TEXT DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    scheduled_for TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_scheduled ON tasks (status, scheduled_for) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tasks_team_id ON tasks (team_id);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks (type);
`
