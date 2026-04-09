# Development Environment

Local development setup that builds sercha-core from source.

## Quick Start

```bash
# Start all services (builds from source)
docker compose up -d --build

# Wait for services to be healthy
docker compose ps

# Run integration tests (from repo root)
cd tests/integration && make test
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| sercha | 8080 | API server (built from source) |
| postgres | 5432 | PostgreSQL with pgvector |
| opensearch | 9200 | OpenSearch for BM25 search |

## UI Development

Run the UI separately with hot reload:

```bash
# From repo root
cd ui
npm install
npm run dev   # http://localhost:3001
```

The Next.js dev server proxies `/api/*` to `localhost:8080` automatically.

## Default Credentials

Set via environment or create via `/api/v1/setup`:
- Use `.env` file for `JWT_SECRET` and `MASTER_KEY`
- First user to hit `/api/v1/setup` becomes admin

## Integration Tests

Integration tests use Cucumber/Gherkin (godog) for BDD-style testing.

```bash
# Run all integration tests (human-readable output)
go test ./tests/integration/... -v

# Run with JSON output (machine-readable, for CI/AI agents)
GODOG_FORMAT=cucumber go test ./tests/integration/... -v

# Run specific feature
go test ./tests/integration/... -v -godog.paths=tests/integration/features/ftue.feature
```

### Features

- `ftue.feature` - First-time user setup (register, login, AI providers)
- `localfs.feature` - LocalFS sync flow (install, source, sync, search)

### Test Requirements

- Docker services must be running
- Tests run against `SERCHA_API_URL` (default: `http://localhost:8080`)

## LocalFS Connector

The dev environment includes a `localfs` connector for testing with local files.
Test data is mounted at `/data/test-docs` inside the container.

### Test Data Structure

```
test-docs/
├── project-alpha/     # Go project with config
│   ├── README.md
│   └── src/
│       ├── main.go
│       └── config.yaml
└── project-beta/      # Python Flask service
    ├── README.md
    ├── app.py
    └── docs/
        └── architecture.md
```

### Using LocalFS via API

```bash
# Create localfs installation
curl -X POST http://localhost:8080/api/v1/installations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Local Test Docs",
    "provider_type": "localfs",
    "api_key": "/data/test-docs"
  }'

# List containers (subdirectories)
curl http://localhost:8080/api/v1/installations/{id}/containers \
  -H "Authorization: Bearer $TOKEN"

# Create source and sync
# ...follow standard source creation flow
```

## Useful Commands

```bash
# View logs
docker compose logs -f sercha

# Rebuild after code changes
docker compose up -d --build sercha

# Stop all services
docker compose down

# Reset everything (including data)
docker compose down -v
```

## Environment Variables

The sercha service uses these environment variables (set in docker-compose.yml):

| Variable | Value | Description |
|----------|-------|-------------|
| DATABASE_URL | postgres://sercha:... | PostgreSQL connection |
| OPENSEARCH_URL | http://opensearch:9200 | OpenSearch for BM25 search |
| PGVECTOR_URL | postgres://sercha:... | pgvector for semantic search |
| PGVECTOR_DIMENSIONS | 1536 | Vector dimensions (OpenAI default) |
| JWT_SECRET | (from .env) | JWT signing secret (64 hex chars) |
| MASTER_KEY | (from .env) | Encryption key (64 hex chars) |
| LOCALFS_ALLOWED_ROOTS | /data | Allowed paths for localfs connector |
