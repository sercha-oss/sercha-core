# Quickstart

Try Sercha with pre-built images. No configuration needed.

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) with at least 4GB RAM.

## Start

```bash
docker compose --profile ui up -d
```

Wait for services to be healthy, then open [http://localhost:3000](http://localhost:3000).

To run without the Admin UI:

```bash
docker compose up -d
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| Admin UI | 3000 | Web interface for managing sources and search |
| API | 8080 | REST API (includes worker and scheduler) |
| PostgreSQL | 5432 | Database with pgvector |
| OpenSearch | 9200 | BM25 text search |

## OAuth Setup

When configuring OAuth providers (e.g. GitHub), use these values:

| Setting | Value |
|---------|-------|
| Callback URL | `http://localhost:8080/api/v1/oauth/callback` |
| Homepage URL | `http://localhost:3000` |

See the [GitHub Connector guide](https://docs.sercha.dev/connectors/github) for step-by-step instructions.

## Stop

```bash
# Preserve data
docker compose --profile ui down

# Remove all data
docker compose --profile ui down -v
```

## Next Steps

- [Documentation](https://docs.sercha.dev/docs/quickstart)
- [API Reference](https://docs.sercha.dev/api/sercha-core-api)
- [Development Setup](../dev/)
