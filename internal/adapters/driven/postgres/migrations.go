package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// migrationsSubdir is the directory within the embedded FS where the .sql
// files live. We pass a sub-FS rooted at this dir to the goose Provider
// so its file collector treats the FS root as the migrations dir.
const migrationsSubdir = "migrations"

// newProvider returns an isolated goose provider for the postgres core
// migration train. WithDisableGlobalRegistry(true) prevents this provider
// from picking up migrations registered by other goose-using packages in
// the binary (notably the pgvector adapter).
func newProvider(db *sql.DB) (*goose.Provider, error) {
	sub, err := fs.Sub(embeddedMigrations, migrationsSubdir)
	if err != nil {
		return nil, fmt.Errorf("sub-fs for migrations: %w", err)
	}
	return goose.NewProvider(
		database.DialectPostgres,
		db,
		sub,
		goose.WithDisableGlobalRegistry(true),
	)
}

// Up applies all embedded up-migrations that have not yet been applied.
// It is idempotent: running it against a fully-migrated DB is a no-op.
func Up(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Down rolls back the most recently applied migration.
func Down(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := p.Down(ctx); err != nil {
		return fmt.Errorf("goose down: %w", err)
	}
	return nil
}

// Status prints applied / pending status of every embedded migration to
// stdout.
func Status(ctx context.Context, db *sql.DB) error {
	p, err := newProvider(db)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("goose status: %w", err)
	}
	for _, s := range statuses {
		state := "pending"
		if s.State == goose.StateApplied {
			state = "applied"
		}
		fmt.Printf("  v%d: %s (%s)\n", s.Source.Version, state, s.Source.Path)
	}
	return nil
}

// Version returns the highest migration version currently recorded as
// applied in the goose_db_version table. Returns 0 against a database
// that has never been migrated.
func Version(ctx context.Context, db *sql.DB) (int64, error) {
	p, err := newProvider(db)
	if err != nil {
		return 0, fmt.Errorf("goose provider: %w", err)
	}
	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("goose version: %w", err)
	}
	return v, nil
}

// MaxEmbeddedVersion returns the highest migration version present in the
// embedded migrations FS. Returns 0 if no migrations are embedded.
//
// Walks the embedded FS directly rather than constructing a goose Provider:
// goose v3.27 Provider requires a non-nil DB even for source enumeration,
// and EnsureClean has to call this from contexts where the DB lives in a
// caller's hand. Filename-parsing is the documented goose convention
// (NNNN_description.sql) and is stable enough for our purposes.
func MaxEmbeddedVersion() (int64, error) {
	entries, err := fs.ReadDir(embeddedMigrations, migrationsSubdir)
	if err != nil {
		return 0, fmt.Errorf("read embedded migrations: %w", err)
	}
	var max int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Goose convention: NNNN_description.sql. Take the version prefix
		// before the first underscore.
		idx := strings.Index(name, "_")
		if idx <= 0 {
			continue
		}
		v, perr := strconv.ParseInt(name[:idx], 10, 64)
		if perr != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

// EnsureClean returns nil if the database's currently-applied migration
// version equals the highest version embedded in the binary.
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
			"schema version mismatch: db at %d, binary expects %d (set AUTO_MIGRATE=true or run `sercha-core migrate up`)",
			got, want,
		)
	}
	return nil
}
