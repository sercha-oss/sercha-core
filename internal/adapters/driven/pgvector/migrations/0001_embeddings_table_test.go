package migrations

import (
	"strings"
	"testing"
)

// TestPgvectorDimensions_DefaultsTo1536 covers the path where
// PGVECTOR_DIMENSIONS is unset — the migration must fall back to 1536 so
// fresh installs match the OpenAI text-embedding-3-small / ada-002 default.
func TestPgvectorDimensions_DefaultsTo1536(t *testing.T) {
	t.Setenv("PGVECTOR_DIMENSIONS", "")
	got, err := pgvectorDimensions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != DefaultDimensions {
		t.Errorf("expected default %d, got %d", DefaultDimensions, got)
	}
}

// TestPgvectorDimensions_HonoursValidEnv verifies that a numeric env value
// overrides the default.
func TestPgvectorDimensions_HonoursValidEnv(t *testing.T) {
	t.Setenv("PGVECTOR_DIMENSIONS", "768")
	got, err := pgvectorDimensions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 768 {
		t.Errorf("expected 768, got %d", got)
	}
}

// TestPgvectorDimensions_RejectsNonNumeric ensures a typo like
// PGVECTOR_DIMENSIONS=foo fails fast with a clear error rather than running
// a bogus CREATE TABLE.
func TestPgvectorDimensions_RejectsNonNumeric(t *testing.T) {
	t.Setenv("PGVECTOR_DIMENSIONS", "abc")
	_, err := pgvectorDimensions()
	if err == nil {
		t.Fatal("expected error for non-numeric env var, got nil")
	}
	if !strings.Contains(err.Error(), "not an integer") {
		t.Errorf("error should mention parse failure, got: %v", err)
	}
}

// TestPgvectorDimensions_RejectsZero covers the lower-bound guard. A zero or
// negative dimension is meaningless for pgvector and would be caught later
// by Postgres anyway, but we want a clearer error sooner.
func TestPgvectorDimensions_RejectsZero(t *testing.T) {
	t.Setenv("PGVECTOR_DIMENSIONS", "0")
	_, err := pgvectorDimensions()
	if err == nil {
		t.Fatal("expected error for zero dimensions, got nil")
	}
}

// TestPgvectorDimensions_RejectsTooLarge covers the upper-bound guard.
// pgvector caps single-precision vectors at 16000 dimensions; rejecting
// values above that fails the migration with a clearer message than the
// generic SQL error you'd get otherwise.
func TestPgvectorDimensions_RejectsTooLarge(t *testing.T) {
	t.Setenv("PGVECTOR_DIMENSIONS", "16001")
	_, err := pgvectorDimensions()
	if err == nil {
		t.Fatal("expected error for over-cap dimensions, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention range limit, got: %v", err)
	}
}

// TestEmbeddings0001_ReturnsMigration is a smoke test for the constructor:
// it must produce a non-nil *goose.Migration with version 1.
func TestEmbeddings0001_ReturnsMigration(t *testing.T) {
	m := Embeddings0001()
	if m == nil {
		t.Fatal("Embeddings0001 returned nil")
	}
	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}
}
