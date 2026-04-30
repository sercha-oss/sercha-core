package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// DB wraps a sql.DB connection pool with Sercha-specific functionality
type DB struct {
	*sql.DB
}

// Config holds database connection-pool tuning. The DSN itself is resolved
// out-of-band via a driven.DBCredentialProvider and is no longer carried on
// this struct.
type Config struct {
	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int

	// ConnMaxLifetime is the maximum lifetime of a connection
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum idle time of a connection
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns sensible defaults for the connection pool.
func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// Connect resolves the DSN via the supplied credential provider, enforces
// the sslmode=disable startup guard, opens a connection pool, and verifies
// connectivity. It does NOT run migrations; that responsibility belongs to
// either the `migrate` subcommand or the AUTO_MIGRATE branch in main.go.
func Connect(ctx context.Context, provider driven.DBCredentialProvider, cfg Config) (*DB, error) {
	if provider == nil {
		return nil, fmt.Errorf("postgres.Connect: nil DBCredentialProvider")
	}

	dsn, err := provider.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve db credential: %w", err)
	}

	// SSL guard: refuse to start against a plaintext DSN unless the operator
	// has explicitly opted in via SERCHA_DEV=1.
	if strings.Contains(dsn, "sslmode=disable") && os.Getenv("SERCHA_DEV") != "1" {
		return nil, fmt.Errorf("sslmode=disable not allowed unless SERCHA_DEV=1")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

// Ping checks if the database is reachable
func (db *DB) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %w, rollback failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// NullString converts a string pointer to sql.NullString
func NullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// NullTime converts a time pointer to sql.NullTime
func NullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// StringPtr converts sql.NullString to string pointer
func StringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// TimePtr converts sql.NullTime to time pointer
func TimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}
