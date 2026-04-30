package main

import (
	"context"
	"testing"
)

// TestRunMigrate_EmptyArgsReturnsUsageError verifies that calling runMigrate
// with no subcommand args returns exit code 2 (usage error). The early
// argument check happens before any DB call, so a nil *sql.DB is safe here.
func TestRunMigrate_EmptyArgsReturnsUsageError(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "core", []string{})
	if rc != 2 {
		t.Errorf("expected exit code 2 for empty args, got %d", rc)
	}
}

// TestRunMigrate_UnknownSubcommandReturnsUsageError verifies that an
// unrecognised subcommand returns exit code 2.
func TestRunMigrate_UnknownSubcommandReturnsUsageError(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "core", []string{"unknown"})
	if rc != 2 {
		t.Errorf("expected exit code 2 for unknown subcommand, got %d", rc)
	}
}

// TestRunMigrate_CreateReturnsZero verifies that the `create` subcommand
// prints help text and returns 0 without ever touching the database for the
// core target.
func TestRunMigrate_CreateReturnsZero(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "core", []string{"create"})
	if rc != 0 {
		t.Errorf("expected exit code 0 for `create`, got %d", rc)
	}
}

// TestRunMigrate_NilArgsReturnsUsageError treats a nil args slice the same as
// an empty one (len(nil) == 0).
func TestRunMigrate_NilArgsReturnsUsageError(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "core", nil)
	if rc != 2 {
		t.Errorf("expected exit code 2 for nil args, got %d", rc)
	}
}

// TestRunMigrate_PgvectorCreateReturnsZero verifies the pgvector target's
// `create` branch dispatches correctly and prints pgvector-specific help.
func TestRunMigrate_PgvectorCreateReturnsZero(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "pgvector", []string{"create"})
	if rc != 0 {
		t.Errorf("expected exit code 0 for `pgvector create`, got %d", rc)
	}
}

// TestRunMigrate_UnknownTargetReturnsUsageError covers the defensive guard in
// opsFor: a target the dispatcher does not recognise (e.g. a typo from a
// future caller) returns a usage-error exit code rather than panicking.
func TestRunMigrate_UnknownTargetReturnsUsageError(t *testing.T) {
	rc := runMigrate(context.Background(), nil, "nonsense", []string{"up"})
	if rc != 2 {
		t.Errorf("expected exit code 2 for unknown target, got %d", rc)
	}
}
