# Development Environment

Builds sercha-core from source. Use this for local development and testing.

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/), [Node.js 20+](https://nodejs.org/), [Go 1.24+](https://go.dev/dl/).

## 1. Configure environment

```bash
cp .env.example .env
```

Generate the required secrets:

```bash
# macOS / Linux
openssl rand -hex 32  # paste as JWT_SECRET
openssl rand -hex 32  # paste as MASTER_KEY
```

Edit `.env` and fill in `JWT_SECRET` and `MASTER_KEY`. The GitHub OAuth and OpenAI fields are optional.

## 2. Start backend services

```bash
docker compose up -d --build
```

This builds sercha-core from source and starts PostgreSQL, OpenSearch, and the API server on port 8080.

## 3. Start the UI

```bash
cd ../../ui
cp .env.example .env.local   # if .env.example exists, otherwise create manually
```

Create `ui/.env.local` with:

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Then install and run:

```bash
npm install
npm run dev
```

The UI runs at [http://localhost:3000](http://localhost:3000).

## Services

| Service | Port | Description |
|---------|------|-------------|
| API | 8080 | sercha-core (built from source) |
| PostgreSQL | 5432 | Database with pgvector |
| OpenSearch | 9200 | BM25 text search |
| UI (dev server) | 3000 | Next.js dev server (run separately) |

## OAuth Setup

When configuring OAuth providers for the dev environment:

| Setting | Value |
|---------|-------|
| Callback URL | `http://localhost:8080/api/v1/oauth/callback` |
| Homepage URL | `http://localhost:3000` |

## Useful commands

```bash
# View API logs
docker compose logs -f sercha

# Rebuild after code changes
docker compose up -d --build sercha

# Stop services (preserve data)
docker compose down

# Stop and remove all data
docker compose down -v
```

## Next Steps

- [Development Guide](https://docs.sercha.dev/docs/development)
- [Configuration Reference](https://docs.sercha.dev/docs/configuration)
- [Contributing](../../CONTRIBUTING.md)
