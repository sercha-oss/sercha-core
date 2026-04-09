# Quickstart

Single container deployment with all dependencies. Ideal for development and small teams.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     With --profile ui                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌──────────────┐         ┌──────────────────────────────┐ │
│   │   Admin UI   │────────▶│       sercha (combined)      │ │
│   │ :3000 (nginx)│         │    API + Worker + Scheduler  │ │
│   └──────────────┘         │          :8080               │ │
│                            └──────────────┬───────────────┘ │
│                                           │                  │
│                         ┌─────────────────┼─────────────────┐│
│                         │                 │                 ││
│                   ┌─────▼─────┐     ┌─────▼─────┐          ││
│                   │ PostgreSQL │     │ OpenSearch│          ││
│                   │   :5432    │     │   :9200   │          ││
│                   │ + pgvector │     │   (BM25)  │          ││
│                   └───────────┘     └───────────┘          ││
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### With Admin UI (Recommended)

```bash
# Start all services including Admin UI
docker compose --profile ui up -d

# Wait for services to be healthy (1-2 minutes)
docker compose --profile ui ps

# Access the Admin UI
open http://localhost:3000
```

### API Only

```bash
# Start services without UI
docker compose up -d

# Wait for services to be healthy
docker compose ps
```

Then use the API to:
- Create an admin user via `POST /api/v1/setup`
- Login via `POST /api/v1/auth/login`
- Configure OAuth providers
- Connect repositories and search

## Services

| Service | Port | Purpose |
|---------|------|---------|
| Admin UI (nginx) | 3000 | Web interface for managing sources and search |
| sercha | 8080 | API server (API + Worker + Scheduler) |
| postgres | 5432 | PostgreSQL with pgvector for vectors |
| opensearch | 9200 | OpenSearch for BM25 text search |

> **Note:** Admin UI is only available when started with `--profile ui`

## OAuth Configuration

When configuring OAuth providers (GitHub, GitLab, etc.) for use with the Admin UI:

| Setting | Value |
|---------|-------|
| Callback URL | `http://localhost:3000/oauth/callback` |
| Homepage URL | `http://localhost:3000` |

## Full Documentation

For detailed documentation including API reference, see the **[Quickstart Guide](https://docs.sercha.dev/core/quickstart)**.

## Stopping Services

```bash
# Stop services (preserves data)
docker compose --profile ui down

# Stop and remove all data
docker compose --profile ui down -v
```
