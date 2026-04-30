package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// TestEnvDBCredential_Resolve_ReturnsValueWhenSet verifies that Resolve
// returns the env var value when it is set.
func TestEnvDBCredential_Resolve_ReturnsValueWhenSet(t *testing.T) {
	const dsn = "postgres://u:p@h:5432/db?sslmode=require"
	t.Setenv("DATABASE_URL", dsn)

	c := NewEnvDBCredential()
	got, err := c.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != dsn {
		t.Errorf("expected DSN %q, got %q", dsn, got)
	}
}

// TestEnvDBCredential_Resolve_ErrorWhenUnset verifies that Resolve returns a
// clear error when the env var is unset (or empty).
func TestEnvDBCredential_Resolve_ErrorWhenUnset(t *testing.T) {
	// Force the var to empty for the duration of this test, regardless of
	// whatever the host environment may have configured. t.Setenv with ""
	// makes os.Getenv treat it as unset.
	t.Setenv("DATABASE_URL", "")

	c := NewEnvDBCredential()
	got, err := c.Resolve(context.Background())
	if err == nil {
		t.Fatalf("expected an error when DATABASE_URL is unset, got DSN %q", got)
	}
	if got != "" {
		t.Errorf("expected empty DSN on error, got %q", got)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("expected error to mention DATABASE_URL, got %v", err)
	}
}

// TestEnvDBCredential_Resolve_HonoursCustomEnvVar verifies that a custom
// EnvVar field is read instead of the default.
func TestEnvDBCredential_Resolve_HonoursCustomEnvVar(t *testing.T) {
	const dsn = "postgres://custom:secret@db:5432/app?sslmode=require"
	// Make sure the default var is not set, so a regression that ignored
	// EnvVar would manifest as an error rather than silently picking up
	// DATABASE_URL.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MY_VAR", dsn)

	c := &EnvDBCredential{EnvVar: "MY_VAR"}
	got, err := c.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != dsn {
		t.Errorf("expected DSN %q, got %q", dsn, got)
	}
}

// TestEnvDBCredential_Resolve_DefaultsToDatabaseURL verifies that when EnvVar
// is the empty string, the implementation falls back to DATABASE_URL.
func TestEnvDBCredential_Resolve_DefaultsToDatabaseURL(t *testing.T) {
	const dsn = "postgres://u:p@h:5432/db?sslmode=require"
	t.Setenv("DATABASE_URL", dsn)

	c := &EnvDBCredential{EnvVar: ""}
	got, err := c.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != dsn {
		t.Errorf("expected DSN %q, got %q", dsn, got)
	}
}

// TestEnvDBCredential_Resolve_DefaultErrorMessageMentionsCustomVar ensures the
// error message names the variable that was actually consulted, not just the
// default. Operators rely on this for diagnostics.
func TestEnvDBCredential_Resolve_DefaultErrorMessageMentionsCustomVar(t *testing.T) {
	t.Setenv("MY_OTHER_VAR", "")

	c := &EnvDBCredential{EnvVar: "MY_OTHER_VAR"}
	_, err := c.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error when MY_OTHER_VAR is unset")
	}
	if !strings.Contains(err.Error(), "MY_OTHER_VAR") {
		t.Errorf("expected error to mention MY_OTHER_VAR, got %v", err)
	}
}

// TestNewEnvDBCredential_DefaultsEnvVarToDATABASE_URL verifies the constructor
// produces a value with EnvVar set to "DATABASE_URL".
func TestNewEnvDBCredential_DefaultsEnvVarToDATABASE_URL(t *testing.T) {
	c := NewEnvDBCredential()
	if c == nil {
		t.Fatal("NewEnvDBCredential returned nil")
	}
	if c.EnvVar != "DATABASE_URL" {
		t.Errorf("expected EnvVar to be \"DATABASE_URL\", got %q", c.EnvVar)
	}
}

// TestEnvDBCredential_ImplementsPort is a compile-time-style check that
// *EnvDBCredential satisfies the driven.DBCredentialProvider port. It pairs
// with the `var _ driven.DBCredentialProvider = (*EnvDBCredential)(nil)`
// assertion in credential.go.
func TestEnvDBCredential_ImplementsPort(t *testing.T) {
	var _ driven.DBCredentialProvider = (*EnvDBCredential)(nil)
	var _ driven.DBCredentialProvider = NewEnvDBCredential()
}
