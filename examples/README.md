# Deployment Examples

Docker Compose configurations for different deployment scenarios.

## Examples

| Example | Description | Use Case |
|---------|-------------|----------|
| [dev](./dev/) | Builds from source (local Dockerfile) | Local development |
| [quickstart](./quickstart/) | Uses pre-built ghcr.io images | Quick setup, production |

## Prerequisites

- Docker and Docker Compose
- 4GB+ RAM recommended

## Quick Start

```bash
# For development (builds from source)
cd examples/dev
docker compose up -d

# For quickstart (pre-built images)
cd examples/quickstart
docker compose up -d
```

## UI Development

The UI is developed separately using Next.js dev server with hot reload:

```bash
# Start backend (from examples/dev)
docker compose up -d

# Start UI dev server (from ui/)
cd ui
npm run dev   # Runs on http://localhost:3001
```

The Next.js dev server automatically proxies `/api/*` requests to the backend at `localhost:8080`.
