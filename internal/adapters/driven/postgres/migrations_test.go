package postgres

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// TestMaxEmbeddedVersion verifies the highest embedded migration version is 1
// (corresponding to migrations/0001_initial_schema.sql). This test is the
// only DB-free test in this file and must always pass.
func TestMaxEmbeddedVersion(t *testing.T) {
	v, err := MaxEmbeddedVersion()
	if err != nil {
		t.Fatalf("MaxEmbeddedVersion returned error: %v", err)
	}
	if v != 1 {
		t.Errorf("expected MaxEmbeddedVersion to be 1, got %d", v)
	}
}

// openTestDB returns a connection to a Postgres instance described by
// TEST_DATABASE_URL, or skips the test when the env var is unset. CI opts in
// by setting TEST_DATABASE_URL.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skipf("set TEST_DATABASE_URL to run")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatalf("ping test db: %v", err)
	}
	return db
}

// resetTestSchema drops the public schema and the goose version table to
// guarantee a clean slate between subtests. The TEST_DATABASE_URL must point
// at a throwaway DB that the test owns end-to-end.
func resetTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		"DROP SCHEMA IF EXISTS public CASCADE",
		"CREATE SCHEMA public",
		"GRANT ALL ON SCHEMA public TO public",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("reset schema (%q): %v", s, err)
		}
	}
}

// TestUp_AppliesMigration0001 verifies that Up against an empty DB applies
// migration 0001 and Version reports 1.
func TestUp_AppliesMigration0001(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	resetTestSchema(t, db)

	ctx := context.Background()
	if err := Up(ctx, db); err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	v, err := Version(ctx, db)
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected version 1, got %d", v)
	}
}

// TestUp_Idempotent verifies that running Up twice on a fresh DB is a no-op
// the second time.
func TestUp_Idempotent(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	resetTestSchema(t, db)

	ctx := context.Background()
	if err := Up(ctx, db); err != nil {
		t.Fatalf("first Up failed: %v", err)
	}
	if err := Up(ctx, db); err != nil {
		t.Fatalf("second Up failed (expected no-op): %v", err)
	}

	v, err := Version(ctx, db)
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected version 1 after two Ups, got %d", v)
	}
}

// TestEnsureClean_NilAfterUp verifies EnsureClean returns nil after Up has
// brought the DB to the highest embedded version.
func TestEnsureClean_NilAfterUp(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	resetTestSchema(t, db)

	ctx := context.Background()
	if err := Up(ctx, db); err != nil {
		t.Fatalf("Up failed: %v", err)
	}
	if err := EnsureClean(ctx, db); err != nil {
		t.Errorf("EnsureClean expected nil after Up, got %v", err)
	}
}

// TestEnsureClean_ErrorOnEmptyDB verifies EnsureClean fails against an empty
// DB (version 0 != embedded version 1) and the error message hints at the
// remediation.
func TestEnsureClean_ErrorOnEmptyDB(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	resetTestSchema(t, db)

	ctx := context.Background()
	err := EnsureClean(ctx, db)
	if err == nil {
		t.Fatal("expected EnsureClean to fail on empty DB, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "schema version mismatch") {
		t.Errorf("expected mismatch error, got %v", err)
	}
}

// TestStatus_NoErrorAfterUp verifies that Status runs to completion against a
// freshly-migrated DB. It writes to stdout via goose's logger, which we
// tolerate; the test only asserts the function returns nil.
func TestStatus_NoErrorAfterUp(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	resetTestSchema(t, db)

	ctx := context.Background()
	if err := Up(ctx, db); err != nil {
		t.Fatalf("Up failed: %v", err)
	}
	if err := Status(ctx, db); err != nil {
		t.Errorf("Status returned error: %v", err)
	}
}
