package domain

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("expected non-empty ID")
	}
	if id2 == "" {
		t.Error("expected non-empty ID")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	// Base64 URL encoding of 16 bytes = 22 chars
	if len(id1) != 22 {
		t.Errorf("expected ID length 22, got %d", len(id1))
	}
}

func TestNewTask(t *testing.T) {
	teamID := "team-123"
	payload := map[string]string{"key": "value"}

	task := NewTask(TaskTypeSyncSource, teamID, payload)

	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.Type != TaskTypeSyncSource {
		t.Errorf("expected type %s, got %s", TaskTypeSyncSource, task.Type)
	}
	if task.TeamID != teamID {
		t.Errorf("expected team ID %s, got %s", teamID, task.TeamID)
	}
	if task.Payload["key"] != "value" {
		t.Error("expected payload to be set")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status %s, got %s", TaskStatusPending, task.Status)
	}
	if task.Priority != 0 {
		t.Errorf("expected priority 0, got %d", task.Priority)
	}
	if task.Attempts != 0 {
		t.Errorf("expected attempts 0, got %d", task.Attempts)
	}
	if task.MaxAttempts != 3 {
		t.Errorf("expected max attempts 3, got %d", task.MaxAttempts)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if task.ScheduledFor.IsZero() {
		t.Error("expected ScheduledFor to be set")
	}
}

func TestNewSyncSourceTask(t *testing.T) {
	teamID := "team-123"
	sourceID := "src-456"

	task := NewSyncSourceTask(teamID, sourceID)

	if task.Type != TaskTypeSyncSource {
		t.Errorf("expected type %s, got %s", TaskTypeSyncSource, task.Type)
	}
	if task.TeamID != teamID {
		t.Errorf("expected team ID %s, got %s", teamID, task.TeamID)
	}
	if task.SourceID() != sourceID {
		t.Errorf("expected source ID %s, got %s", sourceID, task.SourceID())
	}
}

func TestNewSyncAllTask(t *testing.T) {
	teamID := "team-123"

	task := NewSyncAllTask(teamID)

	if task.Type != TaskTypeSyncAll {
		t.Errorf("expected type %s, got %s", TaskTypeSyncAll, task.Type)
	}
	if task.TeamID != teamID {
		t.Errorf("expected team ID %s, got %s", teamID, task.TeamID)
	}
	if task.SourceID() != "" {
		t.Errorf("expected empty source ID, got %s", task.SourceID())
	}
}

func TestTask_SourceID(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]string
		expected string
	}{
		{
			name:     "with source_id",
			payload:  map[string]string{"source_id": "src-123"},
			expected: "src-123",
		},
		{
			name:     "without source_id",
			payload:  map[string]string{"other": "value"},
			expected: "",
		},
		{
			name:     "nil payload",
			payload:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Payload: tt.payload}
			if got := task.SourceID(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestTask_CanRetry(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		maxAttempts int
		expected    bool
	}{
		{"no attempts yet", 0, 3, true},
		{"one attempt", 1, 3, true},
		{"two attempts", 2, 3, true},
		{"max attempts reached", 3, 3, false},
		{"over max attempts", 4, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Attempts: tt.attempts, MaxAttempts: tt.maxAttempts}
			if got := task.CanRetry(); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestTask_IsReady(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name         string
		status       TaskStatus
		scheduledFor time.Time
		expected     bool
	}{
		{"pending and past scheduled", TaskStatusPending, past, true},
		{"pending and future scheduled", TaskStatusPending, future, false},
		{"processing", TaskStatusProcessing, past, false},
		{"completed", TaskStatusCompleted, past, false},
		{"failed", TaskStatusFailed, past, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Status: tt.status, ScheduledFor: tt.scheduledFor}
			if got := task.IsReady(); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestTask_MarkProcessing(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-123", nil)

	task.MarkProcessing()

	if task.Status != TaskStatusProcessing {
		t.Errorf("expected status %s, got %s", TaskStatusProcessing, task.Status)
	}
	if task.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if task.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", task.Attempts)
	}
}

func TestTask_MarkCompleted(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-123", nil)
	task.Error = "some error"

	task.MarkCompleted()

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status %s, got %s", TaskStatusCompleted, task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if task.Error != "" {
		t.Error("expected Error to be cleared")
	}
}

func TestTask_MarkFailed(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-123", nil)
	errorMsg := "something went wrong"

	task.MarkFailed(errorMsg)

	if task.Status != TaskStatusFailed {
		t.Errorf("expected status %s, got %s", TaskStatusFailed, task.Status)
	}
	if task.Error != errorMsg {
		t.Errorf("expected error %s, got %s", errorMsg, task.Error)
	}
}

func TestTask_Retry(t *testing.T) {
	task := NewTask(TaskTypeSyncSource, "team-123", nil)
	task.Attempts = 1
	errorMsg := "retry error"
	beforeRetry := time.Now()

	task.Retry(errorMsg)

	if task.Status != TaskStatusPending {
		t.Errorf("expected status %s, got %s", TaskStatusPending, task.Status)
	}
	if task.Error != errorMsg {
		t.Errorf("expected error %s, got %s", errorMsg, task.Error)
	}
	// With 1 attempt, backoff should be 2^1 = 2 seconds
	expectedBackoff := 2 * time.Second
	expectedScheduledFor := beforeRetry.Add(expectedBackoff)
	if task.ScheduledFor.Before(expectedScheduledFor.Add(-time.Second)) {
		t.Errorf("expected ScheduledFor around %v, got %v", expectedScheduledFor, task.ScheduledFor)
	}
}

func TestTask_Retry_ExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempts        int
		expectedBackoff time.Duration
	}{
		{0, 1 * time.Second},  // 2^0 = 1
		{1, 2 * time.Second},  // 2^1 = 2
		{2, 4 * time.Second},  // 2^2 = 4
		{3, 8 * time.Second},  // 2^3 = 8
		{10, 5 * time.Minute}, // Capped at 5 minutes
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			task := NewTask(TaskTypeSyncSource, "team-123", nil)
			task.Attempts = tt.attempts
			before := time.Now()

			task.Retry("error")

			expectedMin := before.Add(tt.expectedBackoff)
			expectedMax := before.Add(tt.expectedBackoff + time.Second)

			if task.ScheduledFor.Before(expectedMin) || task.ScheduledFor.After(expectedMax) {
				t.Errorf("attempts=%d: expected ScheduledFor between %v and %v, got %v",
					tt.attempts, expectedMin, expectedMax, task.ScheduledFor)
			}
		})
	}
}

func TestScheduledTask_IsDue(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		enabled  bool
		nextRun  time.Time
		expected bool
	}{
		{"enabled and past", true, past, true},
		{"enabled and future", true, future, false},
		{"disabled and past", false, past, false},
		{"disabled and future", false, future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduled := &ScheduledTask{Enabled: tt.enabled, NextRun: tt.nextRun}
			if got := scheduled.IsDue(); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestScheduledTask_UpdateNextRun(t *testing.T) {
	interval := 30 * time.Minute
	scheduled := &ScheduledTask{
		Interval: interval,
	}

	before := time.Now()
	scheduled.UpdateNextRun()
	after := time.Now()

	if scheduled.LastRun == nil {
		t.Error("expected LastRun to be set")
	}
	if scheduled.LastRun.Before(before) || scheduled.LastRun.After(after) {
		t.Error("expected LastRun to be around now")
	}

	expectedNextRun := scheduled.LastRun.Add(interval)
	if scheduled.NextRun != expectedNextRun {
		t.Errorf("expected NextRun %v, got %v", expectedNextRun, scheduled.NextRun)
	}
}

func TestNewScheduledTask(t *testing.T) {
	id := "sched-123"
	name := "Test Schedule"
	taskType := TaskTypeSyncAll
	teamID := "team-456"
	interval := time.Hour

	scheduled := NewScheduledTask(id, name, taskType, teamID, interval)

	if scheduled.ID != id {
		t.Errorf("expected ID %s, got %s", id, scheduled.ID)
	}
	if scheduled.Name != name {
		t.Errorf("expected Name %s, got %s", name, scheduled.Name)
	}
	if scheduled.Type != taskType {
		t.Errorf("expected Type %s, got %s", taskType, scheduled.Type)
	}
	if scheduled.TeamID != teamID {
		t.Errorf("expected TeamID %s, got %s", teamID, scheduled.TeamID)
	}
	if scheduled.Interval != interval {
		t.Errorf("expected Interval %v, got %v", interval, scheduled.Interval)
	}
	if !scheduled.Enabled {
		t.Error("expected Enabled to be true")
	}
	if scheduled.NextRun.IsZero() {
		t.Error("expected NextRun to be set")
	}
}

func TestDefaultSchedulerConfig(t *testing.T) {
	tests := []struct {
		name             string
		teamID           string
		intervalMinutes  int
		expectedInterval time.Duration
	}{
		{
			name:             "default interval (0 defaults to 60)",
			teamID:           "team-123",
			intervalMinutes:  0,
			expectedInterval: 60 * time.Minute,
		},
		{
			name:             "custom interval 30 minutes",
			teamID:           "team-123",
			intervalMinutes:  30,
			expectedInterval: 30 * time.Minute,
		},
		{
			name:             "custom interval 120 minutes",
			teamID:           "team-456",
			intervalMinutes:  120,
			expectedInterval: 120 * time.Minute,
		},
		{
			name:             "negative interval defaults to 60",
			teamID:           "team-789",
			intervalMinutes:  -10,
			expectedInterval: 60 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := DefaultSchedulerConfig(tt.teamID, tt.intervalMinutes)

			if len(configs) == 0 {
				t.Error("expected at least one default config")
			}

			// Check the document sync config
			found := false
			for _, config := range configs {
				if config.ID == "document-sync" {
					found = true
					if config.Type != TaskTypeSyncAll {
						t.Errorf("expected type %s, got %s", TaskTypeSyncAll, config.Type)
					}
					if config.TeamID != tt.teamID {
						t.Errorf("expected team ID %s, got %s", tt.teamID, config.TeamID)
					}
					if config.Interval != tt.expectedInterval {
						t.Errorf("expected interval %v, got %v", tt.expectedInterval, config.Interval)
					}
					if !config.Enabled {
						t.Error("expected Enabled to be true")
					}
				}
			}
			if !found {
				t.Error("expected to find document-sync config")
			}
		})
	}
}

func TestTaskResult(t *testing.T) {
	result := TaskResult{
		TaskID:      "task-123",
		Success:     true,
		Duration:    5 * time.Second,
		ItemsCount:  100,
		ErrorsCount: 2,
	}

	if result.TaskID != "task-123" {
		t.Errorf("expected TaskID task-123, got %s", result.TaskID)
	}
	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Duration != 5*time.Second {
		t.Errorf("expected Duration 5s, got %v", result.Duration)
	}
	if result.ItemsCount != 100 {
		t.Errorf("expected ItemsCount 100, got %d", result.ItemsCount)
	}
	if result.ErrorsCount != 2 {
		t.Errorf("expected ErrorsCount 2, got %d", result.ErrorsCount)
	}
}
