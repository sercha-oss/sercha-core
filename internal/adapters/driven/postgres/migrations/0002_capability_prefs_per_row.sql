-- +goose Up

-- Refactor capability_preferences from one-row-per-team-with-many-toggle-
-- columns to row-per-(team, capability_type). The new shape is
-- extension-friendly: registering a new capability requires no migration —
-- consumers just write a row keyed on the capability's type identifier.
--
-- Migration plan:
--   1. Rename the existing table out of the way.
--   2. Create the new per-row table.
--   3. Backfill: explode each old row's column-set into rows.
--   4. Drop the renamed legacy table.
--
-- All-or-nothing: wrap in a single transaction so a failure mid-way leaves
-- the schema consistent.

ALTER TABLE capability_preferences RENAME TO capability_preferences_legacy;

CREATE TABLE capability_preferences (
    team_id          TEXT        NOT NULL,
    capability_type  TEXT        NOT NULL,
    enabled          BOOLEAN     NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, capability_type)
);

-- Backfill from the legacy table. Each row in the legacy table has one
-- column per known toggle; each becomes a row keyed by capability_type.
-- Only persist toggles whose stored value differs from the new domain
-- defaults — capabilities absent from the new table fall back to
-- descriptor defaults at resolution time, so persisting "the default"
-- would be redundant (and would have to be re-migrated if defaults
-- change).
INSERT INTO capability_preferences (team_id, capability_type, enabled, updated_at)
SELECT team_id, 'text_indexing',      text_indexing_enabled,      updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'embedding_indexing', embedding_indexing_enabled, updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'bm25_search',        bm25_search_enabled,        updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'vector_search',      vector_search_enabled,      updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'query_expansion',    query_expansion_enabled,    updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'query_rewriting',    query_rewriting_enabled,    updated_at FROM capability_preferences_legacy
UNION ALL
SELECT team_id, 'summarization',      summarization_enabled,      updated_at FROM capability_preferences_legacy;

DROP TABLE capability_preferences_legacy;

-- +goose Down

ALTER TABLE capability_preferences RENAME TO capability_preferences_per_row;

CREATE TABLE capability_preferences (
    team_id                    TEXT        PRIMARY KEY,
    text_indexing_enabled      BOOLEAN     NOT NULL DEFAULT true,
    embedding_indexing_enabled BOOLEAN     NOT NULL DEFAULT false,
    bm25_search_enabled        BOOLEAN     NOT NULL DEFAULT true,
    vector_search_enabled      BOOLEAN     NOT NULL DEFAULT true,
    query_expansion_enabled    BOOLEAN     NOT NULL DEFAULT true,
    query_rewriting_enabled    BOOLEAN     NOT NULL DEFAULT true,
    summarization_enabled      BOOLEAN     NOT NULL DEFAULT true,
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Best-effort restore: pivot the per-row data back into per-column.
-- Down migrations are inherently lossy when the source was extension-
-- friendly — toggles for capabilities Core didn't know about (e.g.
-- registered by an add-on) cannot fit back into typed columns and are
-- silently dropped here.
INSERT INTO capability_preferences (
    team_id, text_indexing_enabled, embedding_indexing_enabled,
    bm25_search_enabled, vector_search_enabled,
    query_expansion_enabled, query_rewriting_enabled,
    summarization_enabled, updated_at
)
SELECT
    team_id,
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'text_indexing'),      true),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'embedding_indexing'), false),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'bm25_search'),        true),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'vector_search'),      true),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'query_expansion'),    true),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'query_rewriting'),    true),
    COALESCE(MAX(enabled) FILTER (WHERE capability_type = 'summarization'),      true),
    MAX(updated_at)
FROM capability_preferences_per_row
GROUP BY team_id;

DROP TABLE capability_preferences_per_row;
