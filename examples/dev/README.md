# Development Environment

Local development setup that builds sercha-core from source.

## Quick Start

```bash
# Start all services (builds from source)
docker compose up -d --build

# Run FTUE setup (creates admin, connects Vespa)
./setup.sh

# Run integration tests
./tests/run.sh
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| sercha | 8080 | API server (built from source) |
| postgres | 5432 | PostgreSQL database |
| vespa | 19071 | Vespa config server |

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

## Integration Tests

```bash
# Run all tests
./tests/run.sh

# Run specific test
./tests/run.sh health
./tests/run.sh search
```

## Environment Variables

The sercha service uses these environment variables (set in docker-compose.yml):

| Variable | Value | Description |
|----------|-------|-------------|
| DATABASE_URL | postgres://sercha:sercha_dev@postgres:5432/sercha | PostgreSQL connection |
| VESPA_CONFIG_URL | http://vespa:19071 | Vespa config server |
| VESPA_CONTAINER_URL | http://vespa:8080 | Vespa query endpoint |
| JWT_SECRET | change-me-in-production | JWT signing secret |
| PORT | 8080 | API server port |
