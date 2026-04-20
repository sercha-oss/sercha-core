-- Sercha Core PostgreSQL Schema
-- This schema is idempotent - can be run multiple times safely

-- Enable pgvector extension for vector similarity search
-- Note: Requires pgvector extension to be installed (pgvector/pgvector:pg16 Docker image)
CREATE EXTENSION IF NOT EXISTS vector;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    team_id TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_team_id ON users(team_id);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    refresh_token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_agent TEXT,
    ip_address TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Settings table (team-wide configuration)
CREATE TABLE IF NOT EXISTS settings (
    team_id TEXT PRIMARY KEY,
    ai_provider TEXT,
    embedding_model TEXT,
    ai_endpoint TEXT,
    default_search_mode TEXT NOT NULL DEFAULT 'hybrid',
    results_per_page INT NOT NULL DEFAULT 20,
    max_results_per_page INT NOT NULL DEFAULT 100,
    sync_interval_minutes INT NOT NULL DEFAULT 60,
    sync_enabled BOOLEAN NOT NULL DEFAULT true,
    semantic_search_enabled BOOLEAN NOT NULL DEFAULT true, -- DEPRECATED: use capability_preferences table
    auto_suggest_enabled BOOLEAN NOT NULL DEFAULT true,
    sync_exclusions JSONB DEFAULT '{}' NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT
);

-- AI Settings table (API keys stored separately for security)
CREATE TABLE IF NOT EXISTS ai_settings (
    team_id TEXT PRIMARY KEY,
    embedding_provider TEXT,
    embedding_model TEXT,
    embedding_api_key TEXT,
    embedding_base_url TEXT,
    llm_provider TEXT,
    llm_model TEXT,
    llm_api_key TEXT,
    llm_base_url TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sources table (data sources to index)
CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_sources_provider_type ON sources(provider_type);
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);

-- Documents table (indexed documents from sources)
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    path TEXT,
    title TEXT,
    mime_type TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    indexed_at TIMESTAMPTZ,
    UNIQUE(source_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_documents_source_id ON documents(source_id);
CREATE INDEX IF NOT EXISTS idx_documents_external_id ON documents(external_id);

-- Embeddings table (standalone, managed by pgvector adapter)
-- Decoupled from chunks to avoid storing chunk text in postgres.
-- The pgvector adapter creates this table via EnsureTable() on startup,
-- but we define it here for reference and manual setup.
CREATE TABLE IF NOT EXISTS embeddings (
    chunk_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    embedding vector(1536) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_embeddings_document_id ON embeddings(document_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings USING hnsw (embedding vector_cosine_ops);

-- Sync states table (tracks sync progress per source)
CREATE TABLE IF NOT EXISTS sync_states (
    source_id TEXT PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'idle',
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    cursor TEXT,
    stats JSONB NOT NULL DEFAULT '{}',
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- Scheduled tasks table (recurring task configuration)
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    team_id TEXT NOT NULL,
    interval_ns BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    next_run TIMESTAMPTZ NOT NULL,
    last_run TIMESTAMPTZ,
    last_error TEXT,
    payload JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks(next_run) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_team_id ON scheduled_tasks(team_id);

-- Capability preferences table (stores per-team capability preferences)
CREATE TABLE IF NOT EXISTS capability_preferences (
    team_id TEXT PRIMARY KEY,
    text_indexing_enabled BOOLEAN NOT NULL DEFAULT true,
    embedding_indexing_enabled BOOLEAN NOT NULL DEFAULT false,
    bm25_search_enabled BOOLEAN NOT NULL DEFAULT true,
    vector_search_enabled BOOLEAN NOT NULL DEFAULT true,
    query_expansion_enabled BOOLEAN NOT NULL DEFAULT true,
    query_rewriting_enabled BOOLEAN NOT NULL DEFAULT true,
    summarization_enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add LLM capability columns if they don't exist (for existing databases)
ALTER TABLE capability_preferences ADD COLUMN IF NOT EXISTS query_expansion_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE capability_preferences ADD COLUMN IF NOT EXISTS query_rewriting_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE capability_preferences ADD COLUMN IF NOT EXISTS summarization_enabled BOOLEAN NOT NULL DEFAULT true;

-- Tasks table (task queue for background processing)
-- Used as fallback when Redis is unavailable
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(36) PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    team_id VARCHAR(36) NOT NULL,
    payload JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    error TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for efficient dequeue (pending tasks ordered by priority and schedule)
CREATE INDEX IF NOT EXISTS idx_tasks_dequeue
    ON tasks (status, scheduled_for, priority DESC, created_at)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tasks_team_id ON tasks(team_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Connector installations (OAuth tokens, API keys, PATs)
-- Secrets encrypted at application level (AES-GCM), stored as bytea
CREATE TABLE IF NOT EXISTS connector_installations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    auth_method TEXT NOT NULL,
    secret_blob BYTEA,
    oauth_token_type TEXT,
    oauth_expiry TIMESTAMPTZ,
    oauth_scopes TEXT[],
    account_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    CONSTRAINT unique_provider_account UNIQUE (provider_type, account_id)
);

CREATE INDEX IF NOT EXISTS idx_installations_provider ON connector_installations(provider_type);

-- OAuth state for CSRF protection and PKCE flow
-- Single-use: DELETE ... RETURNING
CREATE TABLE IF NOT EXISTS oauth_states (
    state TEXT PRIMARY KEY,
    provider_type TEXT NOT NULL,
    code_verifier TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    return_context TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oauth_states_expires ON oauth_states(expires_at);

-- Add connection_id and selected_containers to sources table
-- Using DO block to handle idempotent column additions
DO $$
BEGIN
    -- Handle legacy installation_id -> connection_id rename
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sources' AND column_name = 'installation_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sources' AND column_name = 'connection_id'
    ) THEN
        ALTER TABLE sources RENAME COLUMN installation_id TO connection_id;
    END IF;

    -- Add connection_id if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sources' AND column_name = 'connection_id'
    ) THEN
        ALTER TABLE sources ADD COLUMN connection_id TEXT
            REFERENCES connector_installations(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sources' AND column_name = 'selected_containers'
    ) THEN
        ALTER TABLE sources ADD COLUMN selected_containers JSONB DEFAULT '[]';
    END IF;
END $$;

-- Add sync_exclusions column to settings if it doesn't exist (migration)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'settings' AND column_name = 'sync_exclusions'
    ) THEN
        ALTER TABLE settings ADD COLUMN sync_exclusions JSONB DEFAULT '{}' NOT NULL;
    END IF;
END $$;

-- Drop legacy index if it exists and create new one
DROP INDEX IF EXISTS idx_sources_installation_id;
CREATE INDEX IF NOT EXISTS idx_sources_connection_id ON sources(connection_id);

-- Provider configurations (OAuth app credentials, API endpoints)
-- One config per provider type. Multiple installations can use the same config.
-- Secrets encrypted at application level (AES-GCM), stored as bytea
CREATE TABLE IF NOT EXISTS provider_configs (
    provider_type TEXT PRIMARY KEY,
    secret_blob BYTEA,
    auth_url TEXT,
    token_url TEXT,
    scopes TEXT[],
    redirect_uri TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Search queries table (for analytics and tracking)
CREATE TABLE IF NOT EXISTS search_queries (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    query TEXT NOT NULL,
    mode TEXT NOT NULL,
    result_count INT NOT NULL DEFAULT 0,
    duration_ns BIGINT NOT NULL,
    source_ids JSONB DEFAULT '[]',
    has_filters BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_queries_team_id ON search_queries(team_id);
CREATE INDEX IF NOT EXISTS idx_search_queries_created_at ON search_queries(created_at);
CREATE INDEX IF NOT EXISTS idx_search_queries_team_created ON search_queries(team_id, created_at DESC);

-- Platform column for OAuth platform/service separation (Issue #42)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'connector_installations' AND column_name = 'platform'
    ) THEN
        ALTER TABLE connector_installations ADD COLUMN platform TEXT;
        -- Backfill: for existing 1:1 connectors, platform = provider_type
        UPDATE connector_installations SET platform = provider_type WHERE platform IS NULL;
        ALTER TABLE connector_installations ALTER COLUMN platform SET NOT NULL;
    END IF;
END $$;

-- Replace unique constraint: (provider_type, account_id) -> (platform, account_id)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'unique_provider_account'
    ) THEN
        ALTER TABLE connector_installations DROP CONSTRAINT unique_provider_account;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'unique_platform_account'
    ) THEN
        ALTER TABLE connector_installations ADD CONSTRAINT unique_platform_account UNIQUE (platform, account_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_installations_platform ON connector_installations(platform);

-- ===== OAuth 2.0 Authorization Server Tables =====

-- OAuth Clients table (registered third-party applications)
CREATE TABLE IF NOT EXISTS oauth_clients (
    id              TEXT PRIMARY KEY,
    secret_hash     TEXT,
    name            TEXT NOT NULL,
    redirect_uris   TEXT[] NOT NULL,
    grant_types     TEXT[] NOT NULL DEFAULT '{authorization_code}',
    response_types  TEXT[] NOT NULL DEFAULT '{code}',
    scopes          TEXT[] NOT NULL,
    application_type TEXT NOT NULL DEFAULT 'native',
    token_endpoint_auth_method TEXT NOT NULL DEFAULT 'none',
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Authorization Codes table (short-lived codes for auth code flow)
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code            TEXT PRIMARY KEY,
    client_id       TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id),
    redirect_uri    TEXT NOT NULL,
    scopes          TEXT[] NOT NULL,
    code_challenge  TEXT NOT NULL,
    resource        TEXT,
    used            BOOLEAN NOT NULL DEFAULT false,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_authz_codes_expires ON oauth_authorization_codes(expires_at);
CREATE INDEX IF NOT EXISTS idx_oauth_authz_codes_client ON oauth_authorization_codes(client_id);

-- Access Tokens table (short-lived access tokens)
CREATE TABLE IF NOT EXISTS oauth_access_tokens (
    id          TEXT PRIMARY KEY,
    client_id   TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id),
    scopes      TEXT[] NOT NULL,
    audience    TEXT,
    revoked     BOOLEAN NOT NULL DEFAULT false,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_client ON oauth_access_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_user ON oauth_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_expires ON oauth_access_tokens(expires_at);

-- Refresh Tokens table (long-lived refresh tokens with rotation support)
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id              TEXT PRIMARY KEY,
    access_token_id TEXT NOT NULL REFERENCES oauth_access_tokens(id) ON DELETE CASCADE,
    client_id       TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id),
    scopes          TEXT[] NOT NULL,
    audience        TEXT,
    revoked         BOOLEAN NOT NULL DEFAULT false,
    rotated_to      TEXT,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_access ON oauth_refresh_tokens(access_token_id);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_client ON oauth_refresh_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_user ON oauth_refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_expires ON oauth_refresh_tokens(expires_at);

-- Sync events table (for audit logging)
CREATE TABLE IF NOT EXISTS sync_events (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    source_name TEXT,
    provider_type TEXT,
    status TEXT NOT NULL,
    documents_added INT NOT NULL DEFAULT 0,
    documents_updated INT NOT NULL DEFAULT 0,
    documents_deleted INT NOT NULL DEFAULT 0,
    chunks_indexed INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    duration_seconds DECIMAL(10,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_events_team_id ON sync_events(team_id);
CREATE INDEX IF NOT EXISTS idx_sync_events_source_id ON sync_events(source_id);
CREATE INDEX IF NOT EXISTS idx_sync_events_created_at ON sync_events(created_at);
CREATE INDEX IF NOT EXISTS idx_sync_events_team_created ON sync_events(team_id, created_at DESC);
