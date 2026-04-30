package driven

import "context"

// DBCredentialProvider resolves the application's Postgres connection string
// at startup.
//
// It is the seam between the binary and whatever source ultimately holds the
// DB credential. The default implementation reads DATABASE_URL from the
// process environment. Other implementations (HashiCorp Vault, AWS Secrets
// Manager, IAM auth tokens, etc.) are out of scope for this port's package
// but are expected to plug in here without changes to the core or to the
// postgres adapter.
//
// The returned value is an opaque, libpq-compatible DSN. The core does not
// parse or interpret it; only the postgres adapter does. Resolve is called
// exactly once at boot, before sql.Open.
type DBCredentialProvider interface {
	// Resolve returns the full Postgres DSN
	// (e.g. "postgres://user:pass@host:port/db?sslmode=...").
	//
	// Implementations that perform network I/O (secrets managers, IAM)
	// should honour ctx for cancellation and timeouts. Implementations that
	// read purely from process state (env, file) may ignore ctx. A non-nil
	// error indicates the credential could not be resolved and the binary
	// must refuse to start.
	Resolve(ctx context.Context) (string, error)
}
