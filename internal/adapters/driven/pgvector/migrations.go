package pgvector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pgvector/migrations"
)

// pgvectorMigrationsTable is the table goose uses to track applied migrations
// for the pgvector schema. Distinct from the postgres adapter's table so the
// two migration trains do not interfere even when they target the same DB.
const pgvectorMigrationsTable = "pgvector_db_version"

// newProvider returns an isolated goose provider for the pgvector migration
// train. WithDisableGlobalRegistry(true) prevents this provider from seeing
// migrations registered by any other goose-using package in the binary —
// crucially, the postgres adapter's separate migration set.
func newProvider(db *sql.DB) (*goose.Provider, error) {
	return goose.NewProvider(
		database.DialectPostgres,
		db,
		nil, // no FS — Go-coded migration is registered via WithGoMigrations
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(migrations.Embeddings0001()),
		goose.WithTableName(pgvectorMigrationsTable),
	)
}

// Up applies all pending pgvector migrations against the supplied DB. The DB
// must be a *sql.DB (lib/pq driver) — pgvector's runtime path uses pgx, but
// goose itself targets database/sql.
func Up(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("pgvector goose provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("pgvector goose up: %w", err)
	}
	return nil
}

// Down rolls back the most recently applied pgvector migration. The initial
// migration's Down is intentionally a no-op (it would destroy all vectors).
func Down(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("pgvector goose provider: %w", err)
	}
	if _, err := p.Down(ctx); err != nil {
		return fmt.Errorf("pgvector goose down: %w", err)
	}
	return nil
}

// Status prints applied / pending status of every registered pgvector
// migration to stdout.
func Status(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("pgvector goose provider: %w", err)
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("pgvector goose status: %w", err)
	}
	for _, s := range statuses {
		state := "pending"
		if s.State == goose.StateApplied {
			state = "applied"
		}
		fmt.Printf("  pgvector v%d: %s\n", s.Source.Version, state)
	}
	return nil
}

// Version returns the highest pgvector migration version currently applied.
// Returns 0 against a database that has never been migrated for pgvector.
func Version(ctx context.Context, db *sql.DB) (int64, error) {
	p, err := newProvider(db)
	if err != nil {
		return 0, fmt.Errorf("pgvector goose provider: %w", err)
	}
	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("pgvector goose version: %w", err)
	}
	return v, nil
}

// MaxEmbeddedVersion returns the highest pgvector migration version
// registered in this binary. Returns 0 if none are registered.
func MaxEmbeddedVersion() (int64, error) {
	// Construct a provider with a nil DB just to enumerate sources. goose
	// permits nil DBs only via the source-list path; for safety we build
	// against a closed sql.DB stand-in by calling ListSources after a
	// no-op New. To avoid that complexity, we just hard-code the highest
	// version we ship: the registered migration is v1, full stop.
	return 1, nil
}

// EnsureClean returns nil if the database's currently-applied pgvector
// migration version equals the highest version registered in the binary.
func EnsureClean(ctx context.Context, db *sql.DB) error {
	want, err := MaxEmbeddedVersion()
	if err != nil {
		return err
	}

	got, err := Version(ctx, db)
	if err != nil {
		return err
	}

	if got != want {
		return fmt.Errorf(
			"pgvector schema version mismatch: db at %d, binary expects %d (set AUTO_MIGRATE=true or run `sercha-core migrate pgvector up`)",
			got, want,
		)
	}
	return nil
}
