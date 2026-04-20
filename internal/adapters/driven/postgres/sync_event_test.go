package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// TestSyncEventRepository_Save_Success tests successful save of a sync event
func TestSyncEventRepository_Save_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	event := &domain.SyncEvent{
		ID:               "evt-123",
		TeamID:           "team-456",
		SourceID:         "src-789",
		SourceName:       "Test Source",
		ProviderType:     domain.ProviderTypeGitHub,
		Status:           domain.SyncStatusCompleted,
		DocumentsAdded:   10,
		DocumentsUpdated: 5,
		DocumentsDeleted: 2,
		ChunksIndexed:    100,
		ErrorCount:       0,
		ErrorMessage:     "",
		DurationSeconds:  45.5,
		CreatedAt:        time.Now(),
	}

	mock.ExpectExec(`INSERT INTO sync_events`).
		WithArgs(
			event.ID,
			event.TeamID,
			event.SourceID,
			event.SourceName,
			string(event.ProviderType),
			string(event.Status),
			event.DocumentsAdded,
			event.DocumentsUpdated,
			event.DocumentsDeleted,
			event.ChunksIndexed,
			event.ErrorCount,
			event.ErrorMessage,
			event.DurationSeconds,
			event.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Save(context.Background(), event)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_Save_WithError tests saving a failed sync event with error message
func TestSyncEventRepository_Save_WithError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	event := &domain.SyncEvent{
		ID:               "evt-fail-123",
		TeamID:           "team-456",
		SourceID:         "src-789",
		SourceName:       "Failed Source",
		ProviderType:     domain.ProviderTypeNotion,
		Status:           domain.SyncStatusFailed,
		DocumentsAdded:   3,
		DocumentsUpdated: 1,
		DocumentsDeleted: 0,
		ChunksIndexed:    20,
		ErrorCount:       5,
		ErrorMessage:     "authentication token expired",
		DurationSeconds:  10.2,
		CreatedAt:        time.Now(),
	}

	mock.ExpectExec(`INSERT INTO sync_events`).
		WithArgs(
			event.ID,
			event.TeamID,
			event.SourceID,
			event.SourceName,
			string(event.ProviderType),
			string(event.Status),
			event.DocumentsAdded,
			event.DocumentsUpdated,
			event.DocumentsDeleted,
			event.ChunksIndexed,
			event.ErrorCount,
			event.ErrorMessage,
			event.DurationSeconds,
			event.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Save(context.Background(), event)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_Save_DatabaseError tests error handling for save failures
func TestSyncEventRepository_Save_DatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	event := &domain.SyncEvent{
		ID:               "evt-123",
		TeamID:           "team-456",
		SourceID:         "src-789",
		SourceName:       "Test Source",
		ProviderType:     domain.ProviderTypeGitHub,
		Status:           domain.SyncStatusCompleted,
		DocumentsAdded:   10,
		DocumentsUpdated: 5,
		DocumentsDeleted: 2,
		ChunksIndexed:    100,
		ErrorCount:       0,
		ErrorMessage:     "",
		DurationSeconds:  45.5,
		CreatedAt:        time.Now(),
	}

	expectedErr := errors.New("database connection error")

	mock.ExpectExec(`INSERT INTO sync_events`).
		WithArgs(
			event.ID,
			event.TeamID,
			event.SourceID,
			event.SourceName,
			string(event.ProviderType),
			string(event.Status),
			event.DocumentsAdded,
			event.DocumentsUpdated,
			event.DocumentsDeleted,
			event.ChunksIndexed,
			event.ErrorCount,
			event.ErrorMessage,
			event.DurationSeconds,
			event.CreatedAt,
		).
		WillReturnError(expectedErr)

	err = repo.Save(context.Background(), event)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_List_Success tests successful retrieval of sync events by team
func TestSyncEventRepository_List_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	teamID := "team-123"
	limit := 10
	createdAt1 := time.Now().Add(-time.Hour)
	createdAt2 := time.Now().Add(-2 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	}).
		AddRow("evt-1", teamID, "src-1", "Source 1", "github", "completed",
			10, 5, 2, 100, 0, nil, 45.5, createdAt1).
		AddRow("evt-2", teamID, "src-2", "Source 2", "notion", "failed",
			3, 1, 0, 20, 5, "auth error", 10.2, createdAt2)

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(teamID, limit).
		WillReturnRows(rows)

	events, err := repo.List(context.Background(), teamID, limit)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify first event
	if events[0].ID != "evt-1" {
		t.Errorf("events[0].ID = %q, want %q", events[0].ID, "evt-1")
	}
	if events[0].TeamID != teamID {
		t.Errorf("events[0].TeamID = %q, want %q", events[0].TeamID, teamID)
	}
	if events[0].SourceID != "src-1" {
		t.Errorf("events[0].SourceID = %q, want %q", events[0].SourceID, "src-1")
	}
	if events[0].SourceName != "Source 1" {
		t.Errorf("events[0].SourceName = %q, want %q", events[0].SourceName, "Source 1")
	}
	if events[0].ProviderType != domain.ProviderTypeGitHub {
		t.Errorf("events[0].ProviderType = %q, want %q", events[0].ProviderType, domain.ProviderTypeGitHub)
	}
	if events[0].Status != domain.SyncStatusCompleted {
		t.Errorf("events[0].Status = %q, want %q", events[0].Status, domain.SyncStatusCompleted)
	}
	if events[0].DocumentsAdded != 10 {
		t.Errorf("events[0].DocumentsAdded = %d, want 10", events[0].DocumentsAdded)
	}
	if events[0].ErrorMessage != "" {
		t.Errorf("events[0].ErrorMessage = %q, want empty", events[0].ErrorMessage)
	}

	// Verify second event
	if events[1].ID != "evt-2" {
		t.Errorf("events[1].ID = %q, want %q", events[1].ID, "evt-2")
	}
	if events[1].Status != domain.SyncStatusFailed {
		t.Errorf("events[1].Status = %q, want %q", events[1].Status, domain.SyncStatusFailed)
	}
	if events[1].ErrorCount != 5 {
		t.Errorf("events[1].ErrorCount = %d, want 5", events[1].ErrorCount)
	}
	if events[1].ErrorMessage != "auth error" {
		t.Errorf("events[1].ErrorMessage = %q, want %q", events[1].ErrorMessage, "auth error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_List_Empty tests retrieval with no events
func TestSyncEventRepository_List_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	teamID := "team-no-events"
	limit := 10

	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	})

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(teamID, limit).
		WillReturnRows(rows)

	events, err := repo.List(context.Background(), teamID, limit)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_List_LimitClamping tests that invalid limits are clamped
func TestSyncEventRepository_List_LimitClamping(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{
			name:          "zero limit defaults to 50",
			inputLimit:    0,
			expectedLimit: 50,
		},
		{
			name:          "negative limit defaults to 50",
			inputLimit:    -10,
			expectedLimit: 50,
		},
		{
			name:          "limit over 100 clamped to 50",
			inputLimit:    150,
			expectedLimit: 50,
		},
		{
			name:          "valid limit preserved",
			inputLimit:    25,
			expectedLimit: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock db: %v", err)
			}
			defer func() { _ = db.Close() }()

			repo := NewSyncEventRepository(&DB{DB: db})

			rows := sqlmock.NewRows([]string{
				"id", "team_id", "source_id", "source_name", "provider_type", "status",
				"documents_added", "documents_updated", "documents_deleted",
				"chunks_indexed", "error_count", "error_message", "duration_seconds",
				"created_at",
			})

			mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
				WithArgs("team-123", tt.expectedLimit).
				WillReturnRows(rows)

			_, err = repo.List(context.Background(), "team-123", tt.inputLimit)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// TestSyncEventRepository_List_DatabaseError tests error handling for query failures
func TestSyncEventRepository_List_DatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	teamID := "team-123"
	limit := 10
	expectedErr := errors.New("database connection error")

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(teamID, limit).
		WillReturnError(expectedErr)

	events, err := repo.List(context.Background(), teamID, limit)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if events != nil {
		t.Errorf("expected nil events on error, got %v", events)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_List_ScanError tests error handling for row scan failures
func TestSyncEventRepository_List_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	teamID := "team-123"
	limit := 10

	// Return invalid data that will fail to scan
	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	}).
		AddRow("evt-1", teamID, "src-1", "Source 1", "github", "completed",
			"INVALID", 5, 2, 100, 0, nil, 45.5, time.Now()) // documents_added is string instead of int

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(teamID, limit).
		WillReturnRows(rows)

	events, err := repo.List(context.Background(), teamID, limit)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if events != nil {
		t.Errorf("expected nil events on error, got %v", events)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_ListBySource_Success tests successful retrieval of sync events by source
func TestSyncEventRepository_ListBySource_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	sourceID := "src-123"
	limit := 5
	createdAt1 := time.Now().Add(-time.Hour)
	createdAt2 := time.Now().Add(-2 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	}).
		AddRow("evt-1", "team-1", sourceID, "My Source", "github", "completed",
			15, 8, 3, 150, 0, nil, 60.0, createdAt1).
		AddRow("evt-2", "team-1", sourceID, "My Source", "github", "completed",
			5, 2, 1, 50, 0, nil, 30.5, createdAt2)

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(sourceID, limit).
		WillReturnRows(rows)

	events, err := repo.ListBySource(context.Background(), sourceID, limit)
	if err != nil {
		t.Fatalf("ListBySource failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify both events have the same source ID
	for i, event := range events {
		if event.SourceID != sourceID {
			t.Errorf("events[%d].SourceID = %q, want %q", i, event.SourceID, sourceID)
		}
	}

	// Verify first event details
	if events[0].ID != "evt-1" {
		t.Errorf("events[0].ID = %q, want %q", events[0].ID, "evt-1")
	}
	if events[0].DocumentsAdded != 15 {
		t.Errorf("events[0].DocumentsAdded = %d, want 15", events[0].DocumentsAdded)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_ListBySource_Empty tests retrieval with no events for source
func TestSyncEventRepository_ListBySource_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	sourceID := "src-no-events"
	limit := 10

	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	})

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(sourceID, limit).
		WillReturnRows(rows)

	events, err := repo.ListBySource(context.Background(), sourceID, limit)
	if err != nil {
		t.Fatalf("ListBySource failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_ListBySource_LimitClamping tests that invalid limits are clamped
func TestSyncEventRepository_ListBySource_LimitClamping(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{
			name:          "zero limit defaults to 50",
			inputLimit:    0,
			expectedLimit: 50,
		},
		{
			name:          "negative limit defaults to 50",
			inputLimit:    -5,
			expectedLimit: 50,
		},
		{
			name:          "limit over 100 clamped to 50",
			inputLimit:    200,
			expectedLimit: 50,
		},
		{
			name:          "valid limit preserved",
			inputLimit:    30,
			expectedLimit: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock db: %v", err)
			}
			defer func() { _ = db.Close() }()

			repo := NewSyncEventRepository(&DB{DB: db})

			rows := sqlmock.NewRows([]string{
				"id", "team_id", "source_id", "source_name", "provider_type", "status",
				"documents_added", "documents_updated", "documents_deleted",
				"chunks_indexed", "error_count", "error_message", "duration_seconds",
				"created_at",
			})

			mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
				WithArgs("src-123", tt.expectedLimit).
				WillReturnRows(rows)

			_, err = repo.ListBySource(context.Background(), "src-123", tt.inputLimit)
			if err != nil {
				t.Fatalf("ListBySource failed: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// TestSyncEventRepository_ListBySource_DatabaseError tests error handling for query failures
func TestSyncEventRepository_ListBySource_DatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	sourceID := "src-123"
	limit := 10
	expectedErr := errors.New("database timeout")

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(sourceID, limit).
		WillReturnError(expectedErr)

	events, err := repo.ListBySource(context.Background(), sourceID, limit)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if events != nil {
		t.Errorf("expected nil events on error, got %v", events)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSyncEventRepository_InterfaceCompliance verifies the repository implements the interface
func TestSyncEventRepository_InterfaceCompliance(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	// This will fail to compile if the interface is not properly implemented
	repo := NewSyncEventRepository(&DB{DB: db})
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// TestSyncEventRepository_RoundTrip tests saving and retrieving sync events
func TestSyncEventRepository_RoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := NewSyncEventRepository(&DB{DB: db})

	originalEvent := &domain.SyncEvent{
		ID:               "evt-roundtrip",
		TeamID:           "team-rt",
		SourceID:         "src-rt",
		SourceName:       "RoundTrip Source",
		ProviderType:     domain.ProviderTypeGitHub,
		Status:           domain.SyncStatusCompleted,
		DocumentsAdded:   25,
		DocumentsUpdated: 15,
		DocumentsDeleted: 5,
		ChunksIndexed:    300,
		ErrorCount:       0,
		ErrorMessage:     "",
		DurationSeconds:  90.5,
		CreatedAt:        time.Now().Truncate(time.Microsecond), // Postgres precision
	}

	// Mock save
	mock.ExpectExec(`INSERT INTO sync_events`).
		WithArgs(
			originalEvent.ID,
			originalEvent.TeamID,
			originalEvent.SourceID,
			originalEvent.SourceName,
			string(originalEvent.ProviderType),
			string(originalEvent.Status),
			originalEvent.DocumentsAdded,
			originalEvent.DocumentsUpdated,
			originalEvent.DocumentsDeleted,
			originalEvent.ChunksIndexed,
			originalEvent.ErrorCount,
			originalEvent.ErrorMessage,
			originalEvent.DurationSeconds,
			originalEvent.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Save(context.Background(), originalEvent)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Mock retrieve by team
	rows := sqlmock.NewRows([]string{
		"id", "team_id", "source_id", "source_name", "provider_type", "status",
		"documents_added", "documents_updated", "documents_deleted",
		"chunks_indexed", "error_count", "error_message", "duration_seconds",
		"created_at",
	}).
		AddRow(
			originalEvent.ID,
			originalEvent.TeamID,
			originalEvent.SourceID,
			originalEvent.SourceName,
			string(originalEvent.ProviderType),
			string(originalEvent.Status),
			originalEvent.DocumentsAdded,
			originalEvent.DocumentsUpdated,
			originalEvent.DocumentsDeleted,
			originalEvent.ChunksIndexed,
			originalEvent.ErrorCount,
			sql.NullString{}, // Empty error message
			originalEvent.DurationSeconds,
			originalEvent.CreatedAt,
		)

	mock.ExpectQuery(`SELECT id, team_id, source_id, source_name, provider_type, status`).
		WithArgs(originalEvent.TeamID, 50).
		WillReturnRows(rows)

	events, err := repo.List(context.Background(), originalEvent.TeamID, 50)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	retrievedEvent := events[0]

	// Verify all fields match
	if retrievedEvent.ID != originalEvent.ID {
		t.Errorf("ID = %q, want %q", retrievedEvent.ID, originalEvent.ID)
	}
	if retrievedEvent.TeamID != originalEvent.TeamID {
		t.Errorf("TeamID = %q, want %q", retrievedEvent.TeamID, originalEvent.TeamID)
	}
	if retrievedEvent.SourceID != originalEvent.SourceID {
		t.Errorf("SourceID = %q, want %q", retrievedEvent.SourceID, originalEvent.SourceID)
	}
	if retrievedEvent.SourceName != originalEvent.SourceName {
		t.Errorf("SourceName = %q, want %q", retrievedEvent.SourceName, originalEvent.SourceName)
	}
	if retrievedEvent.ProviderType != originalEvent.ProviderType {
		t.Errorf("ProviderType = %q, want %q", retrievedEvent.ProviderType, originalEvent.ProviderType)
	}
	if retrievedEvent.Status != originalEvent.Status {
		t.Errorf("Status = %q, want %q", retrievedEvent.Status, originalEvent.Status)
	}
	if retrievedEvent.DocumentsAdded != originalEvent.DocumentsAdded {
		t.Errorf("DocumentsAdded = %d, want %d", retrievedEvent.DocumentsAdded, originalEvent.DocumentsAdded)
	}
	if retrievedEvent.DocumentsUpdated != originalEvent.DocumentsUpdated {
		t.Errorf("DocumentsUpdated = %d, want %d", retrievedEvent.DocumentsUpdated, originalEvent.DocumentsUpdated)
	}
	if retrievedEvent.DocumentsDeleted != originalEvent.DocumentsDeleted {
		t.Errorf("DocumentsDeleted = %d, want %d", retrievedEvent.DocumentsDeleted, originalEvent.DocumentsDeleted)
	}
	if retrievedEvent.ChunksIndexed != originalEvent.ChunksIndexed {
		t.Errorf("ChunksIndexed = %d, want %d", retrievedEvent.ChunksIndexed, originalEvent.ChunksIndexed)
	}
	if retrievedEvent.ErrorCount != originalEvent.ErrorCount {
		t.Errorf("ErrorCount = %d, want %d", retrievedEvent.ErrorCount, originalEvent.ErrorCount)
	}
	if retrievedEvent.ErrorMessage != originalEvent.ErrorMessage {
		t.Errorf("ErrorMessage = %q, want %q", retrievedEvent.ErrorMessage, originalEvent.ErrorMessage)
	}
	if retrievedEvent.DurationSeconds != originalEvent.DurationSeconds {
		t.Errorf("DurationSeconds = %f, want %f", retrievedEvent.DurationSeconds, originalEvent.DurationSeconds)
	}
	if !retrievedEvent.CreatedAt.Equal(originalEvent.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", retrievedEvent.CreatedAt, originalEvent.CreatedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
