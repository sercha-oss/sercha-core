package postgres

import (
	"context"
	"fmt"
	"os"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// EnvDBCredential resolves the Postgres DSN from a process environment
// variable. The default variable is DATABASE_URL.
//
// EnvDBCredential is the in-binary default impl of driven.DBCredentialProvider.
// Other impls (Vault, AWS Secrets Manager, IAM auth) are out of scope for
// this package but plug into the same port without changes to db.go.
type EnvDBCredential struct {
	// EnvVar names the environment variable to read. If empty, "DATABASE_URL"
	// is used.
	EnvVar string
}

// NewEnvDBCredential returns an EnvDBCredential that reads DATABASE_URL.
func NewEnvDBCredential() *EnvDBCredential {
	return &EnvDBCredential{EnvVar: "DATABASE_URL"}
}

// Resolve reads the configured environment variable and returns its value as
// the DSN. Returns a non-nil error when the variable is unset or empty.
//
// ctx is honoured by the interface contract but ignored here: env lookups
// are non-blocking process-state reads.
func (e *EnvDBCredential) Resolve(_ context.Context) (string, error) {
	name := e.EnvVar
	if name == "" {
		name = "DATABASE_URL"
	}
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("env var %s is not set", name)
	}
	return val, nil
}

// Compile-time port-compliance check.
var _ driven.DBCredentialProvider = (*EnvDBCredential)(nil)
