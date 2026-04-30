-- Sets dev passwords for the application roles. Mounted into Postgres's
-- /docker-entrypoint-initdb.d/ so it runs on first cluster init only.
--
-- The roles themselves are created idempotently by the migrations
-- (0001_initial_schema.sql for sercha_app, the pgvector 0001 Go migration
-- for sercha_vector); this file just ensures the dev compose stack can
-- connect with known passwords.
--
-- For non-dev deployments these passwords are set out-of-band by the
-- operator (or pre-created by IaC) — this bootstrap is dev-compose-only.

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sercha_app') THEN
        CREATE ROLE sercha_app LOGIN PASSWORD 'sercha_app_dev';
    ELSE
        ALTER ROLE sercha_app WITH LOGIN PASSWORD 'sercha_app_dev';
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sercha_vector') THEN
        CREATE ROLE sercha_vector LOGIN PASSWORD 'sercha_vector_dev';
    ELSE
        ALTER ROLE sercha_vector WITH LOGIN PASSWORD 'sercha_vector_dev';
    END IF;
END
$$;
