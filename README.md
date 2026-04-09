<!-- Improved compatibility of back to top link -->
<a id="readme-top"></a>

<!-- PROJECT SHIELDS -->
<p align="center">

[![GitHub Release][release-shield]][release-url]
[![License][license-shield]][license-url]
[![Go Report Card][goreport-shield]][goreport-url]
[![CI Workflow][ci-workflow-shield]][ci-workflow-url]
[![Contributions Welcome][contributions-shield]][contributions-url]

</p>

<!-- BANNER -->
<p align="center">
  <img src=".github/assets/banner.png" alt="Sercha" width="100%">
</p>

<!-- PROJECT TITLE -->
<div align="center">

  <p align="center">
    Self-hosted enterprise search for teams
    <br />
    <br />
    <a href="https://docs.sercha.dev/core">Documentation</a>
    &middot;
    <a href="https://github.com/sercha-oss/sercha-core/issues/new?labels=bug&template=bug_report.md">Report Bug</a>
    &middot;
    <a href="https://github.com/sercha-oss/sercha-core/issues/new?labels=enhancement&template=feature_request.md">Request Feature</a>
  </p>
</div>

<!-- DEMO GIF -->
<p align="center">
  <img src=".github/assets/hero-search-demo.gif" alt="Sercha Demo" width="100%">
</p>

<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#features">Features</a></li>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#quick-start">Quick Start</a></li>
      </ul>
    </li>
    <li><a href="#architecture">Architecture</a></li>
    <li><a href="#deployment-examples">Deployment Examples</a></li>
    <li><a href="#configuration">Configuration</a></li>
    <li><a href="#api-documentation">API Documentation</a></li>
    <li><a href="#development">Development</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
  </ol>
</details>

<!-- ABOUT THE PROJECT -->
## About The Project

Sercha Core is a self-hosted, team-wide search platform that connects your organization's data sources and provides unified search across all of them. Unlike the [Sercha CLI](https://github.com/custodia-labs/sercha-cli) which is designed for individual use, Sercha Core is built for teams with shared data, OAuth authentication, and horizontal scaling.

**Why Sercha Core?**

* **Self-Hosted**: Your data stays on your infrastructure - full control and privacy
* **Team-Oriented**: Shared sources, team management, and role-based access
* **11+ Connectors**: GitHub, GitLab, Slack, Notion, Confluence, Jira, Google Drive, and more
* **Hybrid Search**: BM25 + semantic search with OpenSearch and pgvector
* **Admin UI Included**: Full-featured web interface for managing sources and search
* **Horizontally Scalable**: Deploy as a single container or scale to multiple nodes

<p align="right">(<a href="#readme-top">back to top</a>)</p>

### Features

| Category | Features |
|----------|----------|
| **Data Sources** | GitHub, GitLab, Slack, Notion, Confluence, Jira, Google Drive, Google Docs, Linear, Dropbox, S3 |
| **Search** | Lexical (BM25), semantic embeddings, hybrid search, query expansion |
| **Authentication** | OAuth2 (GitHub, Google, etc.), JWT sessions, team management |
| **API** | RESTful API with OpenAPI/Swagger specification |
| **Deployment** | Docker, Docker Compose, Kubernetes (Helm charts coming soon) |
| **UI** | Admin dashboard for source management, search, and configuration |

<p align="right">(<a href="#readme-top">back to top</a>)</p>

### Built With

* [![Go][Go-badge]][Go-url] - Go 1.24+
* [![PostgreSQL][Postgres-badge]][Postgres-url] - Database + pgvector
* [![OpenSearch][OpenSearch-badge]][OpenSearch-url] - BM25 search
* [![Docker][Docker-badge]][Docker-url] - Containerization

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- GETTING STARTED -->
## Getting Started

### Prerequisites

- Docker and Docker Compose
- 4GB+ RAM recommended
- Ports: 8080 (API), 3000 (UI), 5432 (Postgres), 9200 (OpenSearch)

### Quick Start

The fastest way to get started is using the quickstart example:

```bash
# Clone the repository
git clone https://github.com/sercha-oss/sercha-core.git
cd sercha-core/examples/quickstart

# Start services (with Admin UI)
docker compose --profile ui up -d
```

Wait for services to initialize, then:

- **API**: http://localhost:8080
- **API Docs**: http://localhost:8080/swagger/index.html

For detailed setup instructions, see the [Quickstart Guide](https://docs.sercha.dev/core/quickstart).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ARCHITECTURE -->
## Architecture

Sercha Core uses a hexagonal (ports and adapters) architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                        Sercha Core                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐  │
│   │   REST API  │     │   Worker    │     │  Scheduler  │  │
│   │   Handler   │     │   Tasks     │     │   (Cron)    │  │
│   └──────┬──────┘     └──────┬──────┘     └──────┬──────┘  │
│          │                   │                   │          │
│          └───────────────────┼───────────────────┘          │
│                              │                              │
│                    ┌─────────▼─────────┐                    │
│                    │   Core Services   │                    │
│                    │  (Business Logic) │                    │
│                    └─────────┬─────────┘                    │
│                              │                              │
│      ┌────────────────────┼────────────────────┐            │
│      │                    │                    │            │
│ ┌────▼────┐     ┌─────────▼─────────┐    ┌────▼─────┐      │
│ │PostgreSQL│     │    OpenSearch    │    │Connectors│      │
│ │+pgvector │     │     (BM25)       │    │ (OAuth)  │      │
│ └──────────┘     └──────────────────┘    └──────────┘      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Deployment Modes

| Mode | Use Case | Components |
|------|----------|------------|
| **All-in-One** | Development, small teams | Single binary: API + Worker + Scheduler |

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- DEPLOYMENT EXAMPLES -->
## Deployment Examples

Pre-configured Docker Compose examples are available in the [`examples/`](examples/) directory:

| Example | Description | Use Case |
|---------|-------------|----------|
| [`quickstart/`](examples/quickstart/) | Pre-built images from ghcr.io | Getting started, production |
| [`dev/`](examples/dev/) | Builds from local Dockerfile | Active development |

```bash
# Quickstart (pre-built images)
cd examples/quickstart
docker compose up -d

# Development (builds from source)
cd examples/dev
docker compose up -d
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONFIGURATION -->
## Configuration

Sercha Core is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `JWT_SECRET` | Secret for JWT token signing (64 hex chars) | Required |
| `MASTER_KEY` | Encryption key for secrets (64 hex chars) | Required |
| `OPENSEARCH_URL` | OpenSearch URL for BM25 search | `http://localhost:9200` |
| `PGVECTOR_URL` | PostgreSQL URL for vector search | Same as DATABASE_URL |
| `PORT` | HTTP server port | `8080` |
| `UI_BASE_URL` | Admin UI URL (for OAuth redirects) | `http://localhost:3000` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allowed origins | `http://localhost:3000` |

### AI Configuration (Optional)

| Variable | Description |
|----------|-------------|
| `EMBEDDING_PROVIDER` | Embedding provider (`openai`, `ollama`) |
| `EMBEDDING_MODEL` | Model name for embeddings |
| `EMBEDDING_API_KEY` | API key for embedding provider |
| `LLM_PROVIDER` | LLM provider for query expansion |
| `LLM_MODEL` | Model name for LLM |
| `LLM_API_KEY` | API key for LLM provider |

For complete configuration options, see [Configuration Reference](https://docs.sercha.dev/core/configuration).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- API DOCUMENTATION -->
## API Documentation

Sercha Core provides a full REST API with OpenAPI specification:

- **Swagger UI**: `http://localhost:8080/swagger/index.html` (when running)
- **OpenAPI Spec**: [`swagger/swagger.yaml`](swagger/swagger.yaml)
- **API Reference**: [docs.sercha.dev/core/api_reference](https://docs.sercha.dev/core/api_reference)

### Example: Search API

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "authentication flow", "limit": 10}'
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- DEVELOPMENT -->
## Development

### Prerequisites

- Go 1.24+
- Node.js 20+ (for UI)
- Docker and Docker Compose

### Local Development

```bash
# Terminal 1: Start backend (builds from source)
cd examples/dev
docker compose up -d

# Terminal 2: Start UI with hot reload
cd ui
npm install
npm run dev   # http://localhost:3001
```

The Next.js dev server automatically proxies `/api/*` requests to the backend.

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests
cd tests/integration
make test
```

### Building

```bash
# Backend
go build -o sercha-core ./cmd/sercha-core

# UI (static export)
cd ui && npm run build
```

For detailed development instructions, see [CONTRIBUTING.md](CONTRIBUTING.md).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->
## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting a pull request.

**Please read:**

- [Contributing Guide](CONTRIBUTING.md) - Development workflow and guidelines
- [Security Policy](SECURITY.md) - Reporting security vulnerabilities

### Quick Links

- [Open Issues](https://github.com/sercha-oss/sercha-core/issues)
- [Pull Requests](https://github.com/sercha-oss/sercha-core/pulls)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->
## License

Distributed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[release-shield]: https://img.shields.io/github/v/release/sercha-oss/sercha-core
[release-url]: https://github.com/sercha-oss/sercha-core/releases/latest
[license-shield]: https://img.shields.io/badge/License-Apache_2.0-blue.svg
[license-url]: https://opensource.org/licenses/Apache-2.0
[goreport-shield]: https://goreportcard.com/badge/github.com/sercha-oss/sercha-core
[goreport-url]: https://goreportcard.com/report/github.com/sercha-oss/sercha-core
[ci-workflow-shield]: https://github.com/sercha-oss/sercha-core/actions/workflows/go-ci.yml/badge.svg
[ci-workflow-url]: https://github.com/sercha-oss/sercha-core/actions/workflows/go-ci.yml
[contributions-shield]: https://img.shields.io/badge/contributions-welcome-brightgreen.svg
[contributions-url]: https://github.com/sercha-oss/sercha-core/blob/main/CONTRIBUTING.md
[Go-badge]: https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white
[Go-url]: https://go.dev/
[Postgres-badge]: https://img.shields.io/badge/PostgreSQL-316192?style=flat&logo=postgresql&logoColor=white
[Postgres-url]: https://www.postgresql.org/
[OpenSearch-badge]: https://img.shields.io/badge/OpenSearch-005EB8?style=flat&logo=opensearch&logoColor=white
[OpenSearch-url]: https://opensearch.org/
[Docker-badge]: https://img.shields.io/badge/Docker-2496ED?style=flat&logo=docker&logoColor=white
[Docker-url]: https://www.docker.com/
touch
