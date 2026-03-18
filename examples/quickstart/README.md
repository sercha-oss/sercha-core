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
│                                  ┌────────┴────────┐        │
│                                  │                 │        │
│                            ┌─────▼─────┐     ┌─────▼─────┐  │
│                            │ PostgreSQL │     │   Vespa   │  │
│                            │   :5432    │     │  :19071   │  │
│                            └───────────┘     └───────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### With Admin UI (Recommended)

```bash
# Start all services including Admin UI
docker compose --profile ui up -d

# Wait for services to be healthy (1-2 minutes for Vespa)
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

# Use the interactive setup script
./quickstart.sh
```

The `quickstart.sh` script will guide you through:
- Creating an admin user
- Configuring GitHub OAuth
- Connecting a repository
- Running your first search

### Environment Variables

You can pre-set these to skip the prompts:

```bash
export GITHUB_CLIENT_ID="your-client-id"
export GITHUB_CLIENT_SECRET="your-client-secret"
export ADMIN_EMAIL="admin@example.com"
export ADMIN_PASSWORD="your-password"
./quickstart.sh
```

## Services

| Service | Port | Purpose |
|---------|------|---------|
| Admin UI (nginx) | 3000 | Web interface for managing sources and search |
| sercha | 8080 | API server |
| postgres | 5432 | Database |
| vespa | 19071 | Search engine |

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
