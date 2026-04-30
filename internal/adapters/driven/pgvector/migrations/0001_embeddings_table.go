// Package migrations holds the pgvector schema migrations as Go code so the
// embedding dimension (which must appear as a literal in the column type)
// can be read from PGVECTOR_DIMENSIONS at migrate-time rather than baked
// into a SQL file at compile-time.
package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/pressly/goose/v3"
)

// DefaultDimensions is the OpenAI text-embedding-3-small / ada-002 default.
// Overridable via PGVECTOR_DIMENSIONS at migrate time so the migration matches
// whatever the running adapter is configured for.
const DefaultDimensions = 1536

// Embeddings0001 returns the registered pgvector v1 migration. It is built
// per-call so the Provider can register it with WithGoMigrations and the
// goose package globals stay untouched.
func Embeddings0001() *goose.Migration {
	return goose.NewGoMigration(
		1,
		&goose.GoFunc{RunTx: upEmbeddings},
		&goose.GoFunc{RunTx: downEmbeddings},
	)
}

// pgvectorDimensions reads PGVECTOR_DIMENSIONS or returns the default.
func pgvectorDimensions() (int, error) {
	raw := os.Getenv("PGVECTOR_DIMENSIONS")
	if raw == "" {
		return DefaultDimensions, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("PGVECTOR_DIMENSIONS=%q is not an integer: %w", raw, err)
	}
	// pgvector caps single-precision vectors at 16000 dimensions; reject
	// obviously-wrong values early so the migration fails with a clear
	// message rather than a generic SQL error.
	if n <= 0 || n > 16000 {
		return 0, fmt.Errorf("PGVECTOR_DIMENSIONS=%d out of range (1-16000)", n)
	}
	return n, nil
}

func upEmbeddings(ctx context.Context, tx *sql.Tx) error {
	dim, err := pgvectorDimensions()
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return fmt.Errorf("create vector extension: %w", err)
	}

	createTable := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS embeddings (
            chunk_id TEXT PRIMARY KEY,
            document_id TEXT NOT NULL,
            source_id TEXT NOT NULL DEFAULT '',
            content TEXT NOT NULL DEFAULT '',
            embedding vector(%d) NOT NULL
        )`, dim)
	if _, err := tx.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("create embeddings table: %w", err)
	}

	// Defensive ALTERs covering pre-migration deployments where columns
	// were added incrementally by the legacy EnsureTable path.
	for _, stmt := range []string{
		`ALTER TABLE embeddings ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE embeddings ADD COLUMN IF NOT EXISTS source_id TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_embeddings_document_id ON embeddings(document_id)`,
		`CREATE INDEX IF NOT EXISTS idx_embeddings_source_id ON embeddings(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings USING hnsw (embedding vector_cosine_ops)`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %q: %w", stmt, err)
		}
	}

	// CRUD-only application role. Idempotent; password set out-of-band.
	roleSQL := `
        DO $$
        BEGIN
            IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sercha_vector') THEN
                CREATE ROLE sercha_vector LOGIN;
            END IF;
        END
        $$`
	if _, err := tx.ExecContext(ctx, roleSQL); err != nil {
		return fmt.Errorf("create sercha_vector role: %w", err)
	}

	grantConnect := `
        DO $$
        BEGIN
            EXECUTE format('GRANT CONNECT ON DATABASE %I TO sercha_vector', current_database());
        END
        $$`
	if _, err := tx.ExecContext(ctx, grantConnect); err != nil {
		return fmt.Errorf("grant connect: %w", err)
	}

	for _, stmt := range []string{
		`GRANT USAGE ON SCHEMA public TO sercha_vector`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON embeddings TO sercha_vector`,
		// pgvector's DeleteBySourceAndContainer joins documents to resolve
		// container -> document IDs. Read-only; sercha_vector cannot
		// modify the core schema.
		`GRANT SELECT ON documents TO sercha_vector`,
		// EnsureClean reads pgvector_db_version on every boot to verify
		// the schema is at the expected version. Without this grant the
		// app would fail to start when AUTO_MIGRATE=false and it connects
		// as sercha_vector.
		`GRANT SELECT ON pgvector_db_version TO sercha_vector`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %q: %w", stmt, err)
		}
	}

	return nil
}

// downEmbeddings is intentionally a no-op: rolling back this migration
// would drop the embeddings table and destroy every indexed vector.
// Operators who need to roll back should drop the table by hand.
func downEmbeddings(_ context.Context, _ *sql.Tx) error {
	return nil
}
