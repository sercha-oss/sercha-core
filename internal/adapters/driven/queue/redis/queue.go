package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

const (
	// Stream names
	taskStream     = "sercha:tasks"
	taskGroup      = "sercha:workers"
	scheduledTasks = "sercha:scheduled"

	// Key prefixes
	taskKeyPrefix = "sercha:task:"

	// Default consumer name prefix
	consumerPrefix = "worker-"

	// Claim timeout - how long before a task is considered abandoned
	claimTimeout = 5 * time.Minute
)

// Verify interface compliance
var _ driven.TaskQueue = (*Queue)(nil)

// Queue implements TaskQueue using Redis Streams.
// Redis Streams provide reliable message queuing with consumer groups,
// automatic acknowledgment tracking, and dead letter handling.
type Queue struct {
	client       *redis.Client
	consumerName string
}

// NewQueue creates a new Redis-backed task queue.
// The consumerName should be unique per worker instance (e.g., hostname + PID).
func NewQueue(client *redis.Client, consumerName string) (*Queue, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if consumerName == "" {
		consumerName = fmt.Sprintf("%s%d", consumerPrefix, time.Now().UnixNano())
	}

	q := &Queue{
		client:       client,
		consumerName: consumerName,
	}

	// Create consumer group if it doesn't exist
	ctx := context.Background()
	err := q.client.XGroupCreateMkStream(ctx, taskStream, taskGroup, "0").Err()
	if err != nil && !isGroupExistsError(err) {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return q, nil
}

// Enqueue adds a task to the queue for processing.
func (q *Queue) Enqueue(ctx context.Context, task *domain.Task) error {
	if task == nil {
		return errors.New("task is required")
	}

	// Store full task details in a hash
	taskKey := taskKeyPrefix + task.ID
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Use pipeline for atomic operations
	pipe := q.client.Pipeline()

	// Store task data
	pipe.Set(ctx, taskKey, taskData, 24*time.Hour) // TTL for safety

	// Check if task should be delayed
	if task.ScheduledFor.After(time.Now()) {
		// Add to sorted set for delayed execution
		pipe.ZAdd(ctx, scheduledTasks, redis.Z{
			Score:  float64(task.ScheduledFor.Unix()),
			Member: task.ID,
		})
	} else {
		// Add to stream immediately
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: taskStream,
			Values: map[string]interface{}{
				"task_id":  task.ID,
				"type":     string(task.Type),
				"team_id":  task.TeamID,
				"priority": task.Priority,
			},
		})
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// EnqueueBatch adds multiple tasks to the queue atomically.
func (q *Queue) EnqueueBatch(ctx context.Context, tasks []*domain.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	pipe := q.client.Pipeline()
	now := time.Now()

	for _, task := range tasks {
		if task == nil {
			continue
		}

		taskKey := taskKeyPrefix + task.ID
		taskData, err := json.Marshal(task)
		if err != nil {
			return fmt.Errorf("failed to marshal task %s: %w", task.ID, err)
		}

		pipe.Set(ctx, taskKey, taskData, 24*time.Hour)

		if task.ScheduledFor.After(now) {
			pipe.ZAdd(ctx, scheduledTasks, redis.Z{
				Score:  float64(task.ScheduledFor.Unix()),
				Member: task.ID,
			})
		} else {
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: taskStream,
				Values: map[string]interface{}{
					"task_id":  task.ID,
					"type":     string(task.Type),
					"team_id":  task.TeamID,
					"priority": task.Priority,
				},
			})
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	return nil
}

// Dequeue retrieves the next available task for processing.
// This blocks until a task is available or context is cancelled.
func (q *Queue) Dequeue(ctx context.Context) (*domain.Task, error) {
	return q.DequeueWithTimeout(ctx, 0) // 0 means block forever
}

// DequeueWithTimeout retrieves the next available task, waiting up to timeout seconds.
func (q *Queue) DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error) {
	// First, promote any due scheduled tasks
	if err := q.promoteScheduledTasks(ctx); err != nil {
		// Log but don't fail - this is best effort
		_ = err
	}

	// Try to claim abandoned tasks first
	task, err := q.claimAbandonedTask(ctx)
	if err == nil && task != nil {
		return task, nil
	}

	// Read new tasks from stream
	blockDuration := time.Duration(timeout) * time.Second
	if timeout == 0 {
		blockDuration = 0 // Block forever
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    taskGroup,
		Consumer: q.consumerName,
		Streams:  []string{taskStream, ">"},
		Count:    1,
		Block:    blockDuration,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // No tasks available
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	msg := streams[0].Messages[0]
	taskID, ok := msg.Values["task_id"].(string)
	if !ok {
		// Invalid message, acknowledge and skip
		q.client.XAck(ctx, taskStream, taskGroup, msg.ID)
		return nil, nil
	}

	// Fetch full task data
	task, err = q.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task data: %w", err)
	}

	if task == nil {
		// Task data missing, acknowledge and skip
		q.client.XAck(ctx, taskStream, taskGroup, msg.ID)
		return nil, nil
	}

	// Update task status
	task.MarkProcessing()

	// Store updated task and message ID for ack/nack
	taskData, _ := json.Marshal(task)
	q.client.Set(ctx, taskKeyPrefix+task.ID, taskData, 24*time.Hour)
	q.client.Set(ctx, taskKeyPrefix+task.ID+":msg", msg.ID, 24*time.Hour)

	return task, nil
}

// Ack acknowledges successful completion of a task.
func (q *Queue) Ack(ctx context.Context, taskID string) error {
	// Get the message ID
	msgID, err := q.client.Get(ctx, taskKeyPrefix+taskID+":msg").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get message ID: %w", err)
	}

	pipe := q.client.Pipeline()

	// Acknowledge the message in the stream
	if msgID != "" {
		pipe.XAck(ctx, taskStream, taskGroup, msgID)
		pipe.XDel(ctx, taskStream, msgID)
	}

	// Update task status
	task, err := q.GetTask(ctx, taskID)
	if err == nil && task != nil {
		task.MarkCompleted()
		taskData, _ := json.Marshal(task)
		pipe.Set(ctx, taskKeyPrefix+taskID, taskData, 24*time.Hour)
	}

	// Clean up message ID key
	pipe.Del(ctx, taskKeyPrefix+taskID+":msg")

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to ack task: %w", err)
	}

	return nil
}

// Nack indicates task processing failed and should be retried.
func (q *Queue) Nack(ctx context.Context, taskID string, reason string) error {
	task, err := q.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return errors.New("task not found")
	}

	// Get message ID for acknowledgment
	msgID, _ := q.client.Get(ctx, taskKeyPrefix+taskID+":msg").Result()

	pipe := q.client.Pipeline()

	// Acknowledge the current message (we'll re-enqueue if retrying)
	if msgID != "" {
		pipe.XAck(ctx, taskStream, taskGroup, msgID)
		pipe.XDel(ctx, taskStream, msgID)
	}

	// Check if task can be retried
	if task.CanRetry() {
		task.Retry(reason)
		taskData, _ := json.Marshal(task)
		pipe.Set(ctx, taskKeyPrefix+taskID, taskData, 24*time.Hour)

		// Re-enqueue with delay (via scheduled set)
		pipe.ZAdd(ctx, scheduledTasks, redis.Z{
			Score:  float64(task.ScheduledFor.Unix()),
			Member: task.ID,
		})
	} else {
		// Mark as failed
		task.MarkFailed(reason)
		taskData, _ := json.Marshal(task)
		pipe.Set(ctx, taskKeyPrefix+taskID, taskData, 24*time.Hour)
	}

	// Clean up message ID key
	pipe.Del(ctx, taskKeyPrefix+taskID+":msg")

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to nack task: %w", err)
	}

	return nil
}

// GetTask retrieves a task by ID.
func (q *Queue) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	data, err := q.client.Get(ctx, taskKeyPrefix+taskID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var task domain.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// ListTasks retrieves tasks matching the filter criteria.
// Note: This is less efficient in Redis than Postgres for complex queries.
func (q *Queue) ListTasks(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
	// Scan for task keys (this is O(N) - use sparingly)
	var tasks []*domain.Task
	var cursor uint64
	pattern := taskKeyPrefix + "*"

	for {
		keys, newCursor, err := q.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan tasks: %w", err)
		}

		for _, key := range keys {
			// Skip message ID keys
			if len(key) > 4 && key[len(key)-4:] == ":msg" {
				continue
			}

			data, err := q.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var task domain.Task
			if err := json.Unmarshal([]byte(data), &task); err != nil {
				continue
			}

			// Apply filters
			if filter.TeamID != "" && task.TeamID != filter.TeamID {
				continue
			}
			if filter.Status != "" && task.Status != filter.Status {
				continue
			}
			if filter.Type != "" && task.Type != filter.Type {
				continue
			}

			tasks = append(tasks, &task)

			// Check limit
			if filter.Limit > 0 && len(tasks) >= filter.Limit {
				return tasks, nil
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	// Apply offset (simple slice, not efficient for large offsets)
	if filter.Offset > 0 && filter.Offset < len(tasks) {
		tasks = tasks[filter.Offset:]
	} else if filter.Offset >= len(tasks) {
		return []*domain.Task{}, nil
	}

	return tasks, nil
}

// CancelTask marks a pending task as cancelled.
func (q *Queue) CancelTask(ctx context.Context, taskID string) error {
	task, err := q.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("task not found")
	}

	if task.Status == domain.TaskStatusProcessing {
		return errors.New("cannot cancel task that is processing")
	}
	if task.Status == domain.TaskStatusCompleted || task.Status == domain.TaskStatusFailed {
		return errors.New("cannot cancel completed or failed task")
	}

	pipe := q.client.Pipeline()

	// Remove from scheduled set if present
	pipe.ZRem(ctx, scheduledTasks, taskID)

	// Update task status
	task.Status = domain.TaskStatusFailed
	task.Error = "cancelled"
	task.UpdatedAt = time.Now()
	taskData, _ := json.Marshal(task)
	pipe.Set(ctx, taskKeyPrefix+taskID, taskData, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// PurgeTasks removes completed/failed tasks older than the specified age.
func (q *Queue) PurgeTasks(ctx context.Context, olderThanSeconds int) (int, error) {
	cutoff := time.Now().Add(-time.Duration(olderThanSeconds) * time.Second)
	var purged int

	// Scan for task keys
	var cursor uint64
	pattern := taskKeyPrefix + "*"

	for {
		keys, newCursor, err := q.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return purged, fmt.Errorf("failed to scan tasks: %w", err)
		}

		for _, key := range keys {
			// Skip message ID keys
			if len(key) > 4 && key[len(key)-4:] == ":msg" {
				continue
			}

			data, err := q.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var task domain.Task
			if err := json.Unmarshal([]byte(data), &task); err != nil {
				continue
			}

			// Only purge completed/failed tasks that are old enough
			if (task.Status == domain.TaskStatusCompleted || task.Status == domain.TaskStatusFailed) &&
				task.UpdatedAt.Before(cutoff) {
				q.client.Del(ctx, key)
				purged++
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return purged, nil
}

// Stats returns queue statistics.
func (q *Queue) Stats(ctx context.Context) (*driven.QueueStats, error) {
	stats := &driven.QueueStats{}

	// Get pending count from stream
	info, err := q.client.XInfoStream(ctx, taskStream).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		// Stream might not exist yet
		if !isStreamNotExistsError(err) {
			return nil, fmt.Errorf("failed to get stream info: %w", err)
		}
	} else if err == nil {
		stats.PendingCount = int64(info.Length)
	}

	// Get scheduled count
	scheduledCount, err := q.client.ZCard(ctx, scheduledTasks).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get scheduled count: %w", err)
	}
	stats.PendingCount += scheduledCount

	// Get processing count from consumer group
	groups, err := q.client.XInfoGroups(ctx, taskStream).Result()
	if err == nil {
		for _, group := range groups {
			if group.Name == taskGroup {
				stats.ProcessingCount = int64(group.Pending)
				break
			}
		}
	}

	// Count completed/failed tasks (requires scan - expensive)
	var cursor uint64
	pattern := taskKeyPrefix + "*"

	for {
		keys, newCursor, err := q.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			break
		}

		for _, key := range keys {
			if len(key) > 4 && key[len(key)-4:] == ":msg" {
				continue
			}

			data, _ := q.client.Get(ctx, key).Result()
			var task domain.Task
			if json.Unmarshal([]byte(data), &task) == nil {
				switch task.Status {
				case domain.TaskStatusCompleted:
					stats.CompletedCount++
				case domain.TaskStatusFailed:
					stats.FailedCount++
				}
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return stats, nil
}

// Ping checks if the queue backend is healthy.
func (q *Queue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// Close cleans up resources.
func (q *Queue) Close() error {
	// Redis client is shared, don't close it here
	return nil
}

// promoteScheduledTasks moves due scheduled tasks to the main stream.
func (q *Queue) promoteScheduledTasks(ctx context.Context) error {
	now := time.Now().Unix()

	// Get tasks that are due
	tasks, err := q.client.ZRangeByScore(ctx, scheduledTasks, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		return nil
	}

	pipe := q.client.Pipeline()

	for _, taskID := range tasks {
		// Get task data
		task, err := q.GetTask(ctx, taskID)
		if err != nil || task == nil {
			pipe.ZRem(ctx, scheduledTasks, taskID)
			continue
		}

		// Add to stream
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: taskStream,
			Values: map[string]interface{}{
				"task_id":  task.ID,
				"type":     string(task.Type),
				"team_id":  task.TeamID,
				"priority": task.Priority,
			},
		})

		// Remove from scheduled set
		pipe.ZRem(ctx, scheduledTasks, taskID)
	}

	_, err = pipe.Exec(ctx)
	return err
}

// claimAbandonedTask tries to claim a task that was abandoned by another worker.
func (q *Queue) claimAbandonedTask(ctx context.Context) (*domain.Task, error) {
	// Get pending messages that have been idle too long
	pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: taskStream,
		Group:  taskGroup,
		Start:  "-",
		End:    "+",
		Count:  10,
		Idle:   claimTimeout,
	}).Result()
	if err != nil {
		return nil, err
	}

	for _, p := range pending {
		// Try to claim this message
		claimed, err := q.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   taskStream,
			Group:    taskGroup,
			Consumer: q.consumerName,
			MinIdle:  claimTimeout,
			Messages: []string{p.ID},
		}).Result()
		if err != nil || len(claimed) == 0 {
			continue
		}

		msg := claimed[0]
		taskID, ok := msg.Values["task_id"].(string)
		if !ok {
			// Invalid message, delete it
			q.client.XAck(ctx, taskStream, taskGroup, msg.ID)
			q.client.XDel(ctx, taskStream, msg.ID)
			continue
		}

		task, err := q.GetTask(ctx, taskID)
		if err != nil || task == nil {
			q.client.XAck(ctx, taskStream, taskGroup, msg.ID)
			q.client.XDel(ctx, taskStream, msg.ID)
			continue
		}

		// Update task
		task.MarkProcessing()
		taskData, _ := json.Marshal(task)
		q.client.Set(ctx, taskKeyPrefix+task.ID, taskData, 24*time.Hour)
		q.client.Set(ctx, taskKeyPrefix+task.ID+":msg", msg.ID, 24*time.Hour)

		return task, nil
	}

	return nil, nil
}

// GetJobStats computes aggregated job statistics for a time period
// Note: Redis implementation scans all tasks - for large datasets, consider PostgreSQL
func (q *Queue) GetJobStats(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
	stats := domain.NewJobStats(period)

	// Scan all task keys
	iter := q.client.Scan(ctx, 0, taskKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if key == taskKeyPrefix || key == "" {
			continue
		}

		taskData, err := q.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var task domain.Task
		if err := json.Unmarshal(taskData, &task); err != nil {
			continue
		}

		// Filter by team and period
		if task.TeamID != teamID {
			continue
		}
		if task.CreatedAt.Before(period.Start) || task.CreatedAt.After(period.End) {
			continue
		}

		// Count by status
		stats.TotalJobs++
		switch task.Status {
		case domain.TaskStatusPending:
			stats.PendingJobs++
		case domain.TaskStatusProcessing:
			stats.ProcessingJobs++
		case domain.TaskStatusCompleted:
			stats.CompletedJobs++
		case domain.TaskStatusFailed:
			stats.FailedJobs++
		}

		// Count by type
		stats.JobsByType[task.Type]++

		// Sum retries
		if task.Attempts > 1 {
			stats.TotalRetries += int64(task.Attempts - 1)
		}

		// Calculate duration for completed tasks
		if task.Status == domain.TaskStatusCompleted && task.StartedAt != nil && task.CompletedAt != nil {
			duration := task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
			// Running average
			if stats.AverageDuration == 0 {
				stats.AverageDuration = float64(duration)
			} else {
				stats.AverageDuration = (stats.AverageDuration + float64(duration)) / 2
			}
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan tasks: %w", err)
	}

	stats.CalculateSuccessRate()
	return stats, nil
}

// CountTasks returns the total number of tasks matching the filter
// Note: Redis implementation scans all tasks - for large datasets, consider PostgreSQL
func (q *Queue) CountTasks(ctx context.Context, filter driven.TaskFilter) (int64, error) {
	var count int64

	// Scan all task keys
	iter := q.client.Scan(ctx, 0, taskKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if key == taskKeyPrefix || key == "" {
			continue
		}

		taskData, err := q.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var task domain.Task
		if err := json.Unmarshal(taskData, &task); err != nil {
			continue
		}

		// Filter by team
		if task.TeamID != filter.TeamID {
			continue
		}

		// Filter by status
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}

		// Filter by type
		if filter.Type != "" && task.Type != filter.Type {
			continue
		}

		count++
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("scan tasks: %w", err)
	}

	return count, nil
}

// Helper functions

func isGroupExistsError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

func isStreamNotExistsError(err error) bool {
	return err != nil && (err.Error() == "ERR no such key" ||
		err.Error() == "ERR The XINFO subcommand requires the key to exist")
}
