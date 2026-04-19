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

// TestCapabilityStore_GetPreferences_Found tests successful retrieval of existing preferences
func TestCapabilityStore_GetPreferences_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	teamID := "team-123"
	updatedAt := time.Now()

	rows := sqlmock.NewRows([]string{
		"team_id",
		"text_indexing_enabled",
		"embedding_indexing_enabled",
		"bm25_search_enabled",
		"vector_search_enabled",
		"query_expansion_enabled",
		"query_rewriting_enabled",
		"summarization_enabled",
		"updated_at",
	}).AddRow(teamID, true, false, true, true, true, true, true, updatedAt)

	mock.ExpectQuery(`SELECT team_id, text_indexing_enabled, embedding_indexing_enabled`).
		WithArgs(teamID).
		WillReturnRows(rows)

	prefs, err := store.GetPreferences(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	if prefs.TeamID != teamID {
		t.Errorf("TeamID = %q, want %q", prefs.TeamID, teamID)
	}
	if !prefs.TextIndexingEnabled {
		t.Error("TextIndexingEnabled = false, want true")
	}
	if prefs.EmbeddingIndexingEnabled {
		t.Error("EmbeddingIndexingEnabled = true, want false")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("BM25SearchEnabled = false, want true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("VectorSearchEnabled = false, want true")
	}
	if !prefs.QueryExpansionEnabled {
		t.Error("QueryExpansionEnabled = false, want true")
	}
	if !prefs.QueryRewritingEnabled {
		t.Error("QueryRewritingEnabled = false, want true")
	}
	if !prefs.SummarizationEnabled {
		t.Error("SummarizationEnabled = false, want true")
	}
	if !prefs.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", prefs.UpdatedAt, updatedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_GetPreferences_NotFound tests that default preferences are returned when not found
func TestCapabilityStore_GetPreferences_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	teamID := "team-new"

	mock.ExpectQuery(`SELECT team_id, text_indexing_enabled, embedding_indexing_enabled`).
		WithArgs(teamID).
		WillReturnError(sql.ErrNoRows)

	prefs, err := store.GetPreferences(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	// Verify default preferences are returned
	if prefs.TeamID != teamID {
		t.Errorf("TeamID = %q, want %q", prefs.TeamID, teamID)
	}
	if !prefs.TextIndexingEnabled {
		t.Error("TextIndexingEnabled = false, want true (default)")
	}
	if prefs.EmbeddingIndexingEnabled {
		t.Error("EmbeddingIndexingEnabled = true, want false (default)")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("BM25SearchEnabled = false, want true (default)")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("VectorSearchEnabled = false, want true (default)")
	}
	if !prefs.QueryExpansionEnabled {
		t.Error("QueryExpansionEnabled = false, want true (default)")
	}
	if !prefs.QueryRewritingEnabled {
		t.Error("QueryRewritingEnabled = false, want true (default)")
	}
	if !prefs.SummarizationEnabled {
		t.Error("SummarizationEnabled = false, want true (default)")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_GetPreferences_DatabaseError tests error handling for database errors
func TestCapabilityStore_GetPreferences_DatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	teamID := "team-123"
	expectedErr := errors.New("database connection error")

	mock.ExpectQuery(`SELECT team_id, text_indexing_enabled, embedding_indexing_enabled`).
		WithArgs(teamID).
		WillReturnError(expectedErr)

	prefs, err := store.GetPreferences(context.Background(), teamID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if prefs != nil {
		t.Errorf("expected nil prefs on error, got %v", prefs)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_SavePreferences_Create tests creating new preferences
func TestCapabilityStore_SavePreferences_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	prefs := &domain.CapabilityPreferences{
		TeamID:                   "team-new",
		TextIndexingEnabled:      true,
		EmbeddingIndexingEnabled: true,
		BM25SearchEnabled:        true,
		VectorSearchEnabled:      true,
		QueryExpansionEnabled:    true,
		QueryRewritingEnabled:    true,
		SummarizationEnabled:     true,
		UpdatedAt:                time.Now(),
	}

	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			prefs.TeamID,
			prefs.TextIndexingEnabled,
			prefs.EmbeddingIndexingEnabled,
			prefs.BM25SearchEnabled,
			prefs.VectorSearchEnabled,
			prefs.QueryExpansionEnabled,
			prefs.QueryRewritingEnabled,
			prefs.SummarizationEnabled,
			sqlmock.AnyArg(), // UpdatedAt is set by SavePreferences
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SavePreferences(context.Background(), prefs)
	if err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	// Verify UpdatedAt was updated
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_SavePreferences_Update tests updating existing preferences
func TestCapabilityStore_SavePreferences_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	prefs := &domain.CapabilityPreferences{
		TeamID:                   "team-existing",
		TextIndexingEnabled:      false,
		EmbeddingIndexingEnabled: true,
		BM25SearchEnabled:        false,
		VectorSearchEnabled:      true,
		QueryExpansionEnabled:    false,
		QueryRewritingEnabled:    true,
		SummarizationEnabled:     false,
		UpdatedAt:                time.Now(),
	}

	// The INSERT ... ON CONFLICT DO UPDATE should still work for updates
	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			prefs.TeamID,
			prefs.TextIndexingEnabled,
			prefs.EmbeddingIndexingEnabled,
			prefs.BM25SearchEnabled,
			prefs.VectorSearchEnabled,
			prefs.QueryExpansionEnabled,
			prefs.QueryRewritingEnabled,
			prefs.SummarizationEnabled,
			sqlmock.AnyArg(), // UpdatedAt is set by SavePreferences
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SavePreferences(context.Background(), prefs)
	if err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	// Verify UpdatedAt was updated
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_SavePreferences_DatabaseError tests error handling for save failures
func TestCapabilityStore_SavePreferences_DatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	prefs := &domain.CapabilityPreferences{
		TeamID:                   "team-123",
		TextIndexingEnabled:      true,
		EmbeddingIndexingEnabled: false,
		BM25SearchEnabled:        true,
		VectorSearchEnabled:      false,
		QueryExpansionEnabled:    true,
		QueryRewritingEnabled:    true,
		SummarizationEnabled:     true,
		UpdatedAt:                time.Now(),
	}

	expectedErr := errors.New("database write error")

	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			prefs.TeamID,
			prefs.TextIndexingEnabled,
			prefs.EmbeddingIndexingEnabled,
			prefs.BM25SearchEnabled,
			prefs.VectorSearchEnabled,
			prefs.QueryExpansionEnabled,
			prefs.QueryRewritingEnabled,
			prefs.SummarizationEnabled,
			sqlmock.AnyArg(),
		).
		WillReturnError(expectedErr)

	err = store.SavePreferences(context.Background(), prefs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_SavePreferences_AllFalse tests saving preferences with all capabilities disabled
func TestCapabilityStore_SavePreferences_AllFalse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	prefs := &domain.CapabilityPreferences{
		TeamID:                   "team-disabled",
		TextIndexingEnabled:      false,
		EmbeddingIndexingEnabled: false,
		BM25SearchEnabled:        false,
		VectorSearchEnabled:      false,
		QueryExpansionEnabled:    false,
		QueryRewritingEnabled:    false,
		SummarizationEnabled:     false,
		UpdatedAt:                time.Now(),
	}

	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			prefs.TeamID,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SavePreferences(context.Background(), prefs)
	if err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_SavePreferences_AllTrue tests saving preferences with all capabilities enabled
func TestCapabilityStore_SavePreferences_AllTrue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	prefs := &domain.CapabilityPreferences{
		TeamID:                   "team-enabled",
		TextIndexingEnabled:      true,
		EmbeddingIndexingEnabled: true,
		BM25SearchEnabled:        true,
		VectorSearchEnabled:      true,
		QueryExpansionEnabled:    true,
		QueryRewritingEnabled:    true,
		SummarizationEnabled:     true,
		UpdatedAt:                time.Now(),
	}

	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			prefs.TeamID,
			true,
			true,
			true,
			true,
			true,
			true,
			true,
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SavePreferences(context.Background(), prefs)
	if err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCapabilityStore_RoundTrip tests saving and retrieving preferences
func TestCapabilityStore_RoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewCapabilityStore(&DB{DB: db})

	teamID := "team-roundtrip"
	originalPrefs := &domain.CapabilityPreferences{
		TeamID:                   teamID,
		TextIndexingEnabled:      true,
		EmbeddingIndexingEnabled: false,
		BM25SearchEnabled:        true,
		VectorSearchEnabled:      false,
		QueryExpansionEnabled:    true,
		QueryRewritingEnabled:    false,
		SummarizationEnabled:     true,
		UpdatedAt:                time.Now(),
	}

	// Mock save
	mock.ExpectExec(`INSERT INTO capability_preferences`).
		WithArgs(
			originalPrefs.TeamID,
			originalPrefs.TextIndexingEnabled,
			originalPrefs.EmbeddingIndexingEnabled,
			originalPrefs.BM25SearchEnabled,
			originalPrefs.VectorSearchEnabled,
			originalPrefs.QueryExpansionEnabled,
			originalPrefs.QueryRewritingEnabled,
			originalPrefs.SummarizationEnabled,
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SavePreferences(context.Background(), originalPrefs)
	if err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	// Mock retrieve
	rows := sqlmock.NewRows([]string{
		"team_id",
		"text_indexing_enabled",
		"embedding_indexing_enabled",
		"bm25_search_enabled",
		"vector_search_enabled",
		"query_expansion_enabled",
		"query_rewriting_enabled",
		"summarization_enabled",
		"updated_at",
	}).AddRow(
		originalPrefs.TeamID,
		originalPrefs.TextIndexingEnabled,
		originalPrefs.EmbeddingIndexingEnabled,
		originalPrefs.BM25SearchEnabled,
		originalPrefs.VectorSearchEnabled,
		originalPrefs.QueryExpansionEnabled,
		originalPrefs.QueryRewritingEnabled,
		originalPrefs.SummarizationEnabled,
		originalPrefs.UpdatedAt,
	)

	mock.ExpectQuery(`SELECT team_id, text_indexing_enabled, embedding_indexing_enabled`).
		WithArgs(teamID).
		WillReturnRows(rows)

	retrievedPrefs, err := store.GetPreferences(context.Background(), teamID)
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	// Verify all fields match
	if retrievedPrefs.TeamID != originalPrefs.TeamID {
		t.Errorf("TeamID = %q, want %q", retrievedPrefs.TeamID, originalPrefs.TeamID)
	}
	if retrievedPrefs.TextIndexingEnabled != originalPrefs.TextIndexingEnabled {
		t.Errorf("TextIndexingEnabled = %v, want %v", retrievedPrefs.TextIndexingEnabled, originalPrefs.TextIndexingEnabled)
	}
	if retrievedPrefs.EmbeddingIndexingEnabled != originalPrefs.EmbeddingIndexingEnabled {
		t.Errorf("EmbeddingIndexingEnabled = %v, want %v", retrievedPrefs.EmbeddingIndexingEnabled, originalPrefs.EmbeddingIndexingEnabled)
	}
	if retrievedPrefs.BM25SearchEnabled != originalPrefs.BM25SearchEnabled {
		t.Errorf("BM25SearchEnabled = %v, want %v", retrievedPrefs.BM25SearchEnabled, originalPrefs.BM25SearchEnabled)
	}
	if retrievedPrefs.VectorSearchEnabled != originalPrefs.VectorSearchEnabled {
		t.Errorf("VectorSearchEnabled = %v, want %v", retrievedPrefs.VectorSearchEnabled, originalPrefs.VectorSearchEnabled)
	}
	if retrievedPrefs.QueryExpansionEnabled != originalPrefs.QueryExpansionEnabled {
		t.Errorf("QueryExpansionEnabled = %v, want %v", retrievedPrefs.QueryExpansionEnabled, originalPrefs.QueryExpansionEnabled)
	}
	if retrievedPrefs.QueryRewritingEnabled != originalPrefs.QueryRewritingEnabled {
		t.Errorf("QueryRewritingEnabled = %v, want %v", retrievedPrefs.QueryRewritingEnabled, originalPrefs.QueryRewritingEnabled)
	}
	if retrievedPrefs.SummarizationEnabled != originalPrefs.SummarizationEnabled {
		t.Errorf("SummarizationEnabled = %v, want %v", retrievedPrefs.SummarizationEnabled, originalPrefs.SummarizationEnabled)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
