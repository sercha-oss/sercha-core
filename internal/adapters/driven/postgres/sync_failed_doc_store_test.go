package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// ----- pure helper coverage -----

func TestComputeBackoffDelay_LinearGrowthThenCap(t *testing.T) {
	policy := driven.RetryBackoff{
		Base:        1 * time.Minute,
		Max:         10 * time.Minute,
		MaxAttempts: 100, // doesn't matter for this test
	}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 1 * time.Minute},
		{2, 2 * time.Minute},
		{3, 4 * time.Minute},
		{4, 8 * time.Minute},
		{5, 10 * time.Minute}, // capped
		{6, 10 * time.Minute}, // capped
		{50, 10 * time.Minute},
	}
	for _, tc := range cases {
		if got := computeBackoffDelay(tc.attempt, policy); got != tc.want {
			t.Errorf("attempt %d: got %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestComputeBackoffDelay_AttemptZeroOrNegativeTreatedAsOne(t *testing.T) {
	policy := driven.RetryBackoff{Base: 5 * time.Second, Max: time.Hour, MaxAttempts: 5}
	if got := computeBackoffDelay(0, policy); got != 5*time.Second {
		t.Errorf("attempt=0: got %v, want 5s", got)
	}
	if got := computeBackoffDelay(-3, policy); got != 5*time.Second {
		t.Errorf("attempt=-3: got %v, want 5s", got)
	}
}

func TestTruncateError_NilEmpty(t *testing.T) {
	if got := truncateError(nil, 100); got != "" {
		t.Errorf("nil error: got %q, want empty string", got)
	}
}

func TestTruncateError_KeepsShortMessages(t *testing.T) {
	in := errors.New("short message")
	if got := truncateError(in, 100); got != "short message" {
		t.Errorf("got %q, want %q", got, "short message")
	}
}

func TestTruncateError_TruncatesAtMaxBytes(t *testing.T) {
	in := errors.New(strings.Repeat("a", 4096))
	got := truncateError(in, 100)
	if len([]rune(got)) != 100 {
		t.Errorf("rune length: got %d, want 100", len([]rune(got)))
	}
}

// ----- store behaviour via sqlmock -----

func newStoreWithMock(t *testing.T) (*SyncFailedDocStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	closer := func() { _ = db.Close() }
	return NewSyncFailedDocStore(&DB{DB: db}), mock, closer
}

func TestSyncFailedDocStore_Record_Insert(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	failure := driven.SyncFailedDocRecord{
		SourceID:   "src",
		ExternalID: "ext-1",
		Err:        errors.New("permanently broken"),
		Now:        now,
		Backoff: driven.RetryBackoff{
			Base:        time.Minute,
			Max:         time.Hour,
			MaxAttempts: 5,
		},
	}

	mock.ExpectQuery(`SELECT attempt_count FROM sync_failed_documents`).
		WithArgs("src", "ext-1").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(`INSERT INTO sync_failed_documents`).
		WithArgs(
			"src", "ext-1",
			1,                   // attempt_count
			"permanently broken",
			now,
			now.Add(time.Minute), // 2^0 * Base
			false,               // not terminal
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Record(context.Background(), failure); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestSyncFailedDocStore_Record_BumpsAttemptAndBackoff(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	failure := driven.SyncFailedDocRecord{
		SourceID:   "src",
		ExternalID: "ext-1",
		Err:        errors.New("again"),
		Now:        now,
		Backoff: driven.RetryBackoff{
			Base:        time.Minute,
			Max:         time.Hour,
			MaxAttempts: 5,
		},
	}

	// Existing row at attempt=2 → next attempt is 3, delay is 4min.
	mock.ExpectQuery(`SELECT attempt_count FROM sync_failed_documents`).
		WithArgs("src", "ext-1").
		WillReturnRows(sqlmock.NewRows([]string{"attempt_count"}).AddRow(2))

	mock.ExpectExec(`INSERT INTO sync_failed_documents`).
		WithArgs(
			"src", "ext-1",
			3,                       // bumped
			"again",
			now,
			now.Add(4*time.Minute), // 2^2 * Base
			false,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Record(context.Background(), failure); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestSyncFailedDocStore_Record_TerminalAtMaxAttempts(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	failure := driven.SyncFailedDocRecord{
		SourceID:   "src",
		ExternalID: "ext-1",
		Err:        errors.New("once more"),
		Now:        now,
		Backoff: driven.RetryBackoff{
			Base:        time.Minute,
			Max:         time.Hour,
			MaxAttempts: 3,
		},
	}

	// Existing row at attempt=2 → next attempt 3 == MaxAttempts → terminal.
	mock.ExpectQuery(`SELECT attempt_count FROM sync_failed_documents`).
		WithArgs("src", "ext-1").
		WillReturnRows(sqlmock.NewRows([]string{"attempt_count"}).AddRow(2))

	mock.ExpectExec(`INSERT INTO sync_failed_documents`).
		WithArgs(
			"src", "ext-1",
			3,
			"once more",
			now,
			now.Add(4*time.Minute),
			true, // terminal — attempt count reached MaxAttempts
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Record(context.Background(), failure); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestSyncFailedDocStore_Record_RejectsBadInputs(t *testing.T) {
	store, _, done := newStoreWithMock(t)
	defer done()
	ctx := context.Background()
	cases := []driven.SyncFailedDocRecord{
		{SourceID: "", ExternalID: "ext"},
		{SourceID: "src", ExternalID: ""},
		{SourceID: "src", ExternalID: "ext", Backoff: driven.RetryBackoff{}},
	}
	for i, tc := range cases {
		if err := store.Record(ctx, tc); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

func TestSyncFailedDocStore_MarkSucceeded_DeletesRow(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM sync_failed_documents`).
		WithArgs("src", "ext-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.MarkSucceeded(context.Background(), "src", "ext-1"); err != nil {
		t.Fatalf("MarkSucceeded: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestSyncFailedDocStore_ListReadyForRetry_FiltersAndLimits(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	now := time.Now().UTC()
	// Single row returned — ensures the SQL placeholder count and column
	// ordering match the scan.
	rows := sqlmock.NewRows([]string{
		"source_id", "external_id", "attempt_count", "last_error",
		"last_attempted_at", "next_retry_after", "terminal", "created_at",
	}).AddRow("src", "ext-1", 2, "boom", now, now, false, now)

	mock.ExpectQuery(`SELECT source_id, external_id, attempt_count`).
		WithArgs("src", now, 25).
		WillReturnRows(rows)

	got, err := store.ListReadyForRetry(context.Background(), "src", now, 25)
	if err != nil {
		t.Fatalf("ListReadyForRetry: %v", err)
	}
	if len(got) != 1 || got[0].ExternalID != "ext-1" || got[0].AttemptCount != 2 {
		t.Errorf("unexpected rows: %#v", got)
	}
}

func TestSyncFailedDocStore_ListReadyForRetry_ZeroLimitNoQuery(t *testing.T) {
	store, _, done := newStoreWithMock(t)
	defer done()
	got, err := store.ListReadyForRetry(context.Background(), "src", time.Now(), 0)
	if err != nil {
		t.Fatalf("ListReadyForRetry: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil slice for limit=0; got %v", got)
	}
}

func TestSyncFailedDocStore_CountBySource(t *testing.T) {
	store, mock, done := newStoreWithMock(t)
	defer done()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sync_failed_documents`).
		WithArgs("src").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	n, err := store.CountBySource(context.Background(), "src")
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if n != 7 {
		t.Errorf("got %d, want 7", n)
	}
}
