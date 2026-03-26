# Development Environment

Local development setup that builds sercha-core from source.

## Quick Start

```bash
# Start all services (builds from source)
docker compose up -d --build

# Run FTUE setup (creates admin, connects Vespa)
./setup.sh

# Run integration tests (from repo root)
go test ./tests/integration/... -v
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| sercha | 8080 | API server (built from source) |
| postgres | 5432 | PostgreSQL database |
| vespa | 19071/8080 | Vespa config/query servers |

## With UI

```bash
# Start with web UI
docker compose --profile ui up -d --build

# Access UI at http://localhost:3000
```

## Default Credentials

After running `setup.sh`:
- **Email**: `admin@test.com`
- **Password**: `password123`

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

- `ftue.feature` - First-time user setup (register, login, connect Vespa)
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
| VESPA_CONFIG_URL | http://vespa:19071 | Vespa config server |
| VESPA_CONTAINER_URL | http://vespa:8080 | Vespa query endpoint |
| JWT_SECRET | change-me-in-production | JWT signing secret |
| LOCALFS_ALLOWED_ROOTS | /data | Allowed paths for localfs connector |
