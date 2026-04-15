package domain

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// GenerateID creates a unique random ID.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// TaskType identifies the type of background task
type TaskType string

const (
	// TaskTypeSyncSource syncs a specific source
	TaskTypeSyncSource TaskType = "sync_source"
	// TaskTypeSyncAll syncs all sources for a team
	TaskTypeSyncAll TaskType = "sync_all"
	// TaskTypeSyncContainer syncs a single container within a source
	TaskTypeSyncContainer TaskType = "sync_container"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// Task represents a background job to be processed by workers
type Task struct {
	// ID is the unique identifier for this task
	ID string `json:"id"`

	// Type identifies what kind of task this is
	Type TaskType `json:"type"`

	// TeamID is the team this task belongs to
	TeamID string `json:"team_id"`

	// Payload contains task-specific data
	// For sync_source: {"source_id": "src-123"}
	// For sync_container: {"source_id": "src-123", "container_id": "cnt-456"}
	// For sync_all: {} (empty)
	Payload map[string]string `json:"payload"`

	// Status is the current state of the task
	Status TaskStatus `json:"status"`

	// Priority determines processing order (higher = more urgent)
	// Default is 0, range is -100 to 100
	Priority int `json:"priority"`

	// Attempts is how many times this task has been attempted
	Attempts int `json:"attempts"`

	// MaxAttempts is the maximum retry count before giving up
	MaxAttempts int `json:"max_attempts"`

	// Error contains the last error message if failed
	Error string `json:"error,omitempty"`

	// CreatedAt is when the task was enqueued
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the task was last modified
	UpdatedAt time.Time `json:"updated_at"`

	// StartedAt is when processing began (nil if not started)
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when processing finished (nil if not complete)
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// ScheduledFor is when the task should be processed (for delayed tasks)
	ScheduledFor time.Time `json:"scheduled_for"`
}

// NewTask creates a new task with default values
func NewTask(taskType TaskType, teamID string, payload map[string]string) *Task {
	now := time.Now()
	return &Task{
		ID:           GenerateID(), // Uses existing ID generation
		Type:         taskType,
		TeamID:       teamID,
		Payload:      payload,
		Status:       TaskStatusPending,
		Priority:     0,
		Attempts:     0,
		MaxAttempts:  3,
		CreatedAt:    now,
		UpdatedAt:    now,
		ScheduledFor: now,
	}
}

// NewSyncSourceTask creates a task to sync a specific source
func NewSyncSourceTask(teamID, sourceID string) *Task {
	return NewTask(TaskTypeSyncSource, teamID, map[string]string{
		"source_id": sourceID,
	})
}

// NewSyncAllTask creates a task to sync all sources for a team
func NewSyncAllTask(teamID string) *Task {
	return NewTask(TaskTypeSyncAll, teamID, nil)
}

// NewSyncContainerTask creates a task to sync a single container within a source
func NewSyncContainerTask(teamID, sourceID, containerID string) *Task {
	return NewTask(TaskTypeSyncContainer, teamID, map[string]string{
		"source_id":    sourceID,
		"container_id": containerID,
	})
}

// SourceID extracts the source_id from the payload (for sync_source and sync_container tasks)
func (t *Task) SourceID() string {
	if t.Payload == nil {
		return ""
	}
	return t.Payload["source_id"]
}

// ContainerID extracts the container_id from the payload (for sync_container tasks)
func (t *Task) ContainerID() string {
	if t.Payload == nil {
		return ""
	}
	return t.Payload["container_id"]
}

// CanRetry returns true if the task can be retried
func (t *Task) CanRetry() bool {
	return t.Attempts < t.MaxAttempts
}

// IsReady returns true if the task is ready to be processed
func (t *Task) IsReady() bool {
	return t.Status == TaskStatusPending && time.Now().After(t.ScheduledFor)
}

// MarkProcessing updates the task to processing state
func (t *Task) MarkProcessing() {
	now := time.Now()
	t.Status = TaskStatusProcessing
	t.StartedAt = &now
	t.UpdatedAt = now
	t.Attempts++
}

// MarkCompleted updates the task to completed state
func (t *Task) MarkCompleted() {
	now := time.Now()
	t.Status = TaskStatusCompleted
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Error = ""
}

// MarkFailed updates the task to failed state
func (t *Task) MarkFailed(err string) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.UpdatedAt = now
	t.Error = err
}

// Retry resets the task for retry with exponential backoff
func (t *Task) Retry(err string) {
	now := time.Now()
	t.Status = TaskStatusPending
	t.UpdatedAt = now
	t.Error = err

	// Exponential backoff: 1s, 2s, 4s, 8s, etc.
	backoff := time.Duration(1<<t.Attempts) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute // Cap at 5 minutes
	}
	t.ScheduledFor = now.Add(backoff)
}

// TaskResult represents the outcome of processing a task
type TaskResult struct {
	TaskID      string        `json:"task_id"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	ItemsCount  int           `json:"items_count,omitempty"`  // e.g., documents synced
	ErrorsCount int           `json:"errors_count,omitempty"` // e.g., documents failed
}

// ScheduledTask represents a recurring task configuration
type ScheduledTask struct {
	// ID is the unique identifier for this scheduled task
	ID string `json:"id"`

	// Name is a human-readable name for the task
	Name string `json:"name"`

	// Type is the task type to create when triggered
	Type TaskType `json:"type"`

	// TeamID is the team this schedule belongs to
	TeamID string `json:"team_id"`

	// Interval is how often to run the task
	Interval time.Duration `json:"interval"`

	// Enabled indicates if the schedule is active
	Enabled bool `json:"enabled"`

	// LastRun is when the task was last triggered
	LastRun *time.Time `json:"last_run,omitempty"`

	// NextRun is when the task should next be triggered
	NextRun time.Time `json:"next_run"`

	// LastError contains the last error if the scheduled task failed
	LastError string `json:"last_error,omitempty"`
}

// NewScheduledTask creates a new scheduled task
func NewScheduledTask(id, name string, taskType TaskType, teamID string, interval time.Duration) *ScheduledTask {
	return &ScheduledTask{
		ID:       id,
		Name:     name,
		Type:     taskType,
		TeamID:   teamID,
		Interval: interval,
		Enabled:  true,
		NextRun:  time.Now().Add(interval),
	}
}

// IsDue returns true if the scheduled task should be triggered
func (s *ScheduledTask) IsDue() bool {
	return s.Enabled && time.Now().After(s.NextRun)
}

// UpdateNextRun calculates the next run time after execution
func (s *ScheduledTask) UpdateNextRun() {
	now := time.Now()
	s.LastRun = &now
	s.NextRun = now.Add(s.Interval)
}

// DefaultSchedulerConfig returns the default scheduled tasks with custom interval
// If intervalMinutes is 0, defaults to 60 minutes
func DefaultSchedulerConfig(teamID string, intervalMinutes int) []*ScheduledTask {
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}

	return []*ScheduledTask{
		NewScheduledTask(
			"document-sync",
			"Document Sync",
			TaskTypeSyncAll,
			teamID,
			time.Duration(intervalMinutes)*time.Minute,
		),
	}
}
