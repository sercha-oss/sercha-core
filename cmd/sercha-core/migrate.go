package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pgvector"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/postgres"
)

// runMigrateCmd wires the migrate subcommand. Two targets are supported:
//
//   - core    (default): the postgres adapter's migrations against DATABASE_URL.
//     Invoked as `sercha-core migrate <up|down|...>`.
//   - pgvector: the pgvector adapter's migrations against PGVECTOR_URL.
//     Invoked as `sercha-core migrate pgvector <up|down|...>`.
//
// Each target opens its own database/sql handle. SSL guard fires for both.
func runMigrateCmd(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	target := "core"
	if len(args) > 0 && args[0] == "pgvector" {
		target = "pgvector"
		args = args[1:]
	}

	envVar := "DATABASE_URL"
	if target == "pgvector" {
		envVar = "PGVECTOR_URL"
	}

	dsn := os.Getenv(envVar)
	if dsn == "" {
		fmt.Fprintf(os.Stderr, "migrate: env var %s is not set\n", envVar)
		return 1
	}

	// Apply the same SSL guard the running binary applies. Operators who run
	// `migrate` against a plaintext local DB must opt in explicitly.
	if strings.Contains(dsn, "sslmode=disable") && os.Getenv("SERCHA_DEV") != "1" {
		fmt.Fprintln(os.Stderr, "migrate: sslmode=disable not allowed unless SERCHA_DEV=1")
		return 1
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: open db: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: ping db: %v\n", err)
		return 1
	}

	return runMigrate(ctx, db, target, args)
}

// migrateOps abstracts the per-target set of migration operations so
// runMigrate can dispatch generically to either the postgres or pgvector
// adapter without a switch on every subcommand.
type migrateOps struct {
	up      func(context.Context, *sql.DB) error
	down    func(context.Context, *sql.DB) error
	status  func(context.Context, *sql.DB) error
	version func(context.Context, *sql.DB) (int64, error)
	// createHelp is printed by the `create` subcommand. Each adapter has a
	// different on-disk migration layout and conventions.
	createHelp string
}

func opsFor(target string) (migrateOps, error) {
	switch target {
	case "core":
		return migrateOps{
			up:      postgres.Up,
			down:    postgres.Down,
			status:  postgres.Status,
			version: postgres.Version,
			createHelp: `To create a new core migration:

  1. Pick the next sequential 4-digit number after the highest existing file
     under internal/adapters/driven/postgres/migrations/ (e.g. 0002).
  2. Create internal/adapters/driven/postgres/migrations/NNNN_description.sql.
  3. Begin the file with "-- +goose Up" and wrap any multi-statement chunks
     (DO blocks, CREATE FUNCTION bodies) with "-- +goose StatementBegin"
     / "-- +goose StatementEnd" markers.
  4. Optionally add a "-- +goose Down" section.
  5. Rebuild the binary so go:embed picks up the new file.`,
		}, nil
	case "pgvector":
		return migrateOps{
			up:      pgvector.Up,
			down:    pgvector.Down,
			status:  pgvector.Status,
			version: pgvector.Version,
			createHelp: `To create a new pgvector migration:

  1. Add a Go file under internal/adapters/driven/pgvector/migrations/
     named NNNN_description.go (next sequential version after the highest
     existing file).
  2. Construct the migration with goose.NewGoMigration and expose it via a
     package-level constructor (e.g. EmbeddingsNNNN()).
  3. Register it in internal/adapters/driven/pgvector/migrations.go's
     newProvider via WithGoMigrations.
  4. Rebuild the binary; the migration is registered at provider construction.`,
		}, nil
	default:
		return migrateOps{}, fmt.Errorf("unknown migrate target: %s", target)
	}
}

// runMigrate dispatches a `migrate` subcommand against the named target.
// It expects to be called with the args AFTER the target keyword, e.g.
// for `sercha-core migrate pgvector up`, target is "pgvector" and args is
// []string{"up"}. Returns the desired process exit code.
//
// Supported subcommands (per target):
//   - up:      apply all pending migrations
//   - down:    roll back the last applied migration
//   - status:  print applied/pending status of every registered migration
//   - version: print the current applied schema version
//   - create:  print instructions for authoring a new migration
func runMigrate(ctx context.Context, db *sql.DB, target string, args []string) int {
	usage := func() {
		fmt.Fprintln(os.Stderr, "usage: sercha-core migrate [pgvector] <up|down|status|version|create>")
	}

	if len(args) == 0 {
		usage()
		return 2
	}

	ops, err := opsFor(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		usage()
		return 2
	}

	sub := args[0]
	label := target + " "
	if target == "core" {
		label = ""
	}

	switch sub {
	case "up":
		if err := ops.up(ctx, db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sup failed: %v\n", label, err)
			return 1
		}
		v, err := ops.version(ctx, db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sup: applied, but version read failed: %v\n", label, err)
			return 1
		}
		fmt.Printf("%smigrations applied (current version: %d)\n", label, v)
		return 0

	case "down":
		if err := ops.down(ctx, db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sdown failed: %v\n", label, err)
			return 1
		}
		v, err := ops.version(ctx, db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sdown: rolled back, but version read failed: %v\n", label, err)
			return 1
		}
		fmt.Printf("%smigration rolled back (current version: %d)\n", label, v)
		return 0

	case "status":
		if err := ops.status(ctx, db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sstatus failed: %v\n", label, err)
			return 1
		}
		return 0

	case "version":
		v, err := ops.version(ctx, db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate %sversion failed: %v\n", label, err)
			return 1
		}
		fmt.Printf("%d\n", v)
		return 0

	case "create":
		fmt.Println(ops.createHelp)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown migrate subcommand: %s\n", sub)
		usage()
		return 2
	}
}
