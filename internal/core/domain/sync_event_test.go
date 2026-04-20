package domain

import (
	"testing"
	"time"
)

func TestNewSyncEvent(t *testing.T) {
	teamID := "team-123"
	sourceID := "source-456"
	sourceName := "Test Source"
	providerType := ProviderTypeGitHub
	status := SyncStatusCompleted
	stats := SyncStats{
		DocumentsAdded:   10,
		DocumentsUpdated: 5,
		DocumentsDeleted: 2,
		ChunksIndexed:    100,
		Errors:           0,
	}
	duration := 45.5

	event := NewSyncEvent(teamID, sourceID, sourceName, providerType, status, stats, duration)

	if event.ID == "" {
		t.Error("expected non-empty ID")
	}
	if event.TeamID != teamID {
		t.Errorf("expected team ID %s, got %s", teamID, event.TeamID)
	}
	if event.SourceID != sourceID {
		t.Errorf("expected source ID %s, got %s", sourceID, event.SourceID)
	}
	if event.SourceName != sourceName {
		t.Errorf("expected source name %s, got %s", sourceName, event.SourceName)
	}
	if event.ProviderType != providerType {
		t.Errorf("expected provider type %s, got %s", providerType, event.ProviderType)
	}
	if event.Status != status {
		t.Errorf("expected status %s, got %s", status, event.Status)
	}
	if event.DocumentsAdded != stats.DocumentsAdded {
		t.Errorf("expected documents added %d, got %d", stats.DocumentsAdded, event.DocumentsAdded)
	}
	if event.DocumentsUpdated != stats.DocumentsUpdated {
		t.Errorf("expected documents updated %d, got %d", stats.DocumentsUpdated, event.DocumentsUpdated)
	}
	if event.DocumentsDeleted != stats.DocumentsDeleted {
		t.Errorf("expected documents deleted %d, got %d", stats.DocumentsDeleted, event.DocumentsDeleted)
	}
	if event.ChunksIndexed != stats.ChunksIndexed {
		t.Errorf("expected chunks indexed %d, got %d", stats.ChunksIndexed, event.ChunksIndexed)
	}
	if event.ErrorCount != stats.Errors {
		t.Errorf("expected error count %d, got %d", stats.Errors, event.ErrorCount)
	}
	if event.DurationSeconds != duration {
		t.Errorf("expected duration %f, got %f", duration, event.DurationSeconds)
	}
	if event.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if event.ErrorMessage != "" {
		t.Error("expected empty error message for successful sync")
	}
}

func TestNewSyncEvent_WithErrors(t *testing.T) {
	stats := SyncStats{
		DocumentsAdded:   8,
		DocumentsUpdated: 3,
		DocumentsDeleted: 1,
		ChunksIndexed:    50,
		Errors:           3,
	}

	event := NewSyncEvent("team-1", "src-1", "Source", ProviderTypeNotion, SyncStatusCompleted, stats, 60.0)

	if event.ErrorCount != 3 {
		t.Errorf("expected error count 3, got %d", event.ErrorCount)
	}
	if event.ErrorMessage != "" {
		t.Error("expected empty error message (set via WithError)")
	}
}

func TestSyncEvent_WithError(t *testing.T) {
	event := NewSyncEvent(
		"team-1",
		"src-1",
		"Source",
		ProviderTypeGitHub,
		SyncStatusFailed,
		SyncStats{Errors: 1},
		30.0,
	)

	errorMsg := "failed to fetch documents"
	result := event.WithError(errorMsg)

	if result.ErrorMessage != errorMsg {
		t.Errorf("expected error message %s, got %s", errorMsg, result.ErrorMessage)
	}
	// Verify it returns the same instance
	if result != event {
		t.Error("expected WithError to return the same instance")
	}
}

func TestSyncEvent_IsSuccessful(t *testing.T) {
	tests := []struct {
		name     string
		status   SyncStatus
		expected bool
	}{
		{
			name:     "completed status is successful",
			status:   SyncStatusCompleted,
			expected: true,
		},
		{
			name:     "failed status is not successful",
			status:   SyncStatusFailed,
			expected: false,
		},
		{
			name:     "running status is not successful",
			status:   SyncStatusRunning,
			expected: false,
		},
		{
			name:     "idle status is not successful",
			status:   SyncStatusIdle,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SyncEvent{Status: tt.status}
			if got := event.IsSuccessful(); got != tt.expected {
				t.Errorf("expected IsSuccessful() = %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestSyncEvent_IsFailed(t *testing.T) {
	tests := []struct {
		name     string
		status   SyncStatus
		expected bool
	}{
		{
			name:     "failed status is failed",
			status:   SyncStatusFailed,
			expected: true,
		},
		{
			name:     "completed status is not failed",
			status:   SyncStatusCompleted,
			expected: false,
		},
		{
			name:     "running status is not failed",
			status:   SyncStatusRunning,
			expected: false,
		},
		{
			name:     "idle status is not failed",
			status:   SyncStatusIdle,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SyncEvent{Status: tt.status}
			if got := event.IsFailed(); got != tt.expected {
				t.Errorf("expected IsFailed() = %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestSyncEvent_TotalDocuments(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		updated  int
		deleted  int
		expected int
	}{
		{
			name:     "all zeros",
			added:    0,
			updated:  0,
			deleted:  0,
			expected: 0,
		},
		{
			name:     "only added",
			added:    10,
			updated:  0,
			deleted:  0,
			expected: 10,
		},
		{
			name:     "only updated",
			added:    0,
			updated:  5,
			deleted:  0,
			expected: 5,
		},
		{
			name:     "only deleted",
			added:    0,
			updated:  0,
			deleted:  3,
			expected: 3,
		},
		{
			name:     "mixed documents",
			added:    10,
			updated:  5,
			deleted:  2,
			expected: 17,
		},
		{
			name:     "large numbers",
			added:    1000,
			updated:  500,
			deleted:  200,
			expected: 1700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SyncEvent{
				DocumentsAdded:   tt.added,
				DocumentsUpdated: tt.updated,
				DocumentsDeleted: tt.deleted,
			}
			if got := event.TotalDocuments(); got != tt.expected {
				t.Errorf("expected TotalDocuments() = %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestSyncEvent_GetDuration(t *testing.T) {
	tests := []struct {
		name            string
		durationSeconds float64
		expected        time.Duration
	}{
		{
			name:            "zero duration",
			durationSeconds: 0.0,
			expected:        0,
		},
		{
			name:            "one second",
			durationSeconds: 1.0,
			expected:        time.Second,
		},
		{
			name:            "fractional seconds",
			durationSeconds: 1.5,
			expected:        1500 * time.Millisecond,
		},
		{
			name:            "one minute",
			durationSeconds: 60.0,
			expected:        time.Minute,
		},
		{
			name:            "large duration",
			durationSeconds: 3600.0,
			expected:        time.Hour,
		},
		{
			name:            "milliseconds precision",
			durationSeconds: 0.123,
			expected:        123 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SyncEvent{DurationSeconds: tt.durationSeconds}
			got := event.GetDuration()
			if got != tt.expected {
				t.Errorf("expected GetDuration() = %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestSyncEvent_CompleteSyncScenario(t *testing.T) {
	// Simulate a complete successful sync
	teamID := "team-prod-123"
	sourceID := "github-repo-456"
	sourceName := "MyOrg/MyRepo"
	providerType := ProviderTypeGitHub
	stats := SyncStats{
		DocumentsAdded:   25,
		DocumentsUpdated: 10,
		DocumentsDeleted: 3,
		ChunksIndexed:    500,
		Errors:           0,
	}
	duration := 120.5

	event := NewSyncEvent(teamID, sourceID, sourceName, providerType, SyncStatusCompleted, stats, duration)

	// Verify event structure
	if !event.IsSuccessful() {
		t.Error("expected successful sync")
	}
	if event.IsFailed() {
		t.Error("expected not failed")
	}
	if event.TotalDocuments() != 38 {
		t.Errorf("expected total documents 38, got %d", event.TotalDocuments())
	}
	if event.GetDuration() != 120*time.Second+500*time.Millisecond {
		t.Errorf("expected duration 120.5s, got %v", event.GetDuration())
	}
	if event.ErrorMessage != "" {
		t.Error("expected no error message for successful sync")
	}
}

func TestSyncEvent_FailedSyncScenario(t *testing.T) {
	// Simulate a failed sync
	teamID := "team-dev-789"
	sourceID := "notion-workspace-012"
	sourceName := "Dev Workspace"
	providerType := ProviderTypeNotion
	stats := SyncStats{
		DocumentsAdded:   5,
		DocumentsUpdated: 2,
		DocumentsDeleted: 0,
		ChunksIndexed:    50,
		Errors:           10,
	}
	duration := 45.2
	errorMsg := "authentication token expired"

	event := NewSyncEvent(teamID, sourceID, sourceName, providerType, SyncStatusFailed, stats, duration).
		WithError(errorMsg)

	// Verify event structure
	if event.IsSuccessful() {
		t.Error("expected not successful")
	}
	if !event.IsFailed() {
		t.Error("expected failed sync")
	}
	if event.ErrorCount != 10 {
		t.Errorf("expected error count 10, got %d", event.ErrorCount)
	}
	if event.ErrorMessage != errorMsg {
		t.Errorf("expected error message %s, got %s", errorMsg, event.ErrorMessage)
	}
	if event.TotalDocuments() != 7 {
		t.Errorf("expected total documents 7, got %d", event.TotalDocuments())
	}
}

func TestSyncEvent_ZeroStatsScenario(t *testing.T) {
	// Simulate a sync with no changes (all documents up to date)
	event := NewSyncEvent(
		"team-1",
		"src-1",
		"No Changes Source",
		ProviderTypeGitHub,
		SyncStatusCompleted,
		SyncStats{
			DocumentsAdded:   0,
			DocumentsUpdated: 0,
			DocumentsDeleted: 0,
			ChunksIndexed:    0,
			Errors:           0,
		},
		10.0,
	)

	if !event.IsSuccessful() {
		t.Error("expected successful sync even with zero stats")
	}
	if event.TotalDocuments() != 0 {
		t.Errorf("expected total documents 0, got %d", event.TotalDocuments())
	}
	if event.ErrorCount != 0 {
		t.Errorf("expected error count 0, got %d", event.ErrorCount)
	}
}

func TestSyncEvent_IDUniqueness(t *testing.T) {
	// Verify that each new event gets a unique ID
	event1 := NewSyncEvent("team-1", "src-1", "Source 1", ProviderTypeGitHub, SyncStatusCompleted, SyncStats{}, 10.0)
	event2 := NewSyncEvent("team-1", "src-1", "Source 1", ProviderTypeGitHub, SyncStatusCompleted, SyncStats{}, 10.0)

	if event1.ID == "" {
		t.Error("expected non-empty ID for event1")
	}
	if event2.ID == "" {
		t.Error("expected non-empty ID for event2")
	}
	if event1.ID == event2.ID {
		t.Error("expected unique IDs for different events")
	}
	// Base64 URL encoding of 16 bytes = 22 chars (same as GenerateID)
	if len(event1.ID) != 22 {
		t.Errorf("expected ID length 22, got %d", len(event1.ID))
	}
}

func TestSyncEvent_CreatedAtTimestamp(t *testing.T) {
	before := time.Now()
	event := NewSyncEvent("team-1", "src-1", "Source", ProviderTypeGitHub, SyncStatusCompleted, SyncStats{}, 10.0)
	after := time.Now()

	if event.CreatedAt.Before(before) || event.CreatedAt.After(after.Add(time.Second)) {
		t.Error("expected CreatedAt to be close to now")
	}
}
