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
    <a href="https://github.com/custodia-labs/sercha-core/issues/new?labels=bug&template=bug_report.md">Report Bug</a>
    &middot;
    <a href="https://github.com/custodia-labs/sercha-core/issues/new?labels=enhancement&template=feature_request.md">Request Feature</a>
  </p>
</div>

<!-- DEMO GIF -->
<p align="center">
  <img src="docs/assets/hero-search-demo.gif" alt="Sercha Demo" width="100%">
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
* **Hybrid Search**: Semantic search powered by Vespa with optional AI enhancements
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
* [![PostgreSQL][Postgres-badge]][Postgres-url] - Database
* [![Vespa][Vespa-badge]][Vespa-url] - Search engine
* [![Docker][Docker-badge]][Docker-url] - Containerization

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- GETTING STARTED -->
## Getting Started

### Prerequisites

- Docker and Docker Compose
- 4GB+ RAM (Vespa requires memory)
- Ports: 8080 (API), 3000 (UI), 5432 (Postgres), 19071 (Vespa)

### Quick Start

The fastest way to get started is using the quickstart example:

```bash
# Clone the repository
git clone https://github.com/custodia-labs/sercha-core.git
cd sercha-core/examples/quickstart

# Start with Admin UI (recommended)
docker compose --profile ui up -d

# Or API only
docker compose up -d
```

Wait 1-2 minutes for Vespa to initialize, then:

- **Admin UI**: http://localhost:3000
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
│          ┌───────────────────┼───────────────────┐          │
│          │                   │                   │          │
│   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐  │
│   │  PostgreSQL │     │    Vespa    │     │ Connectors  │  │
│   │  (Storage)  │     │  (Search)   │     │  (OAuth)    │  │
│   └─────────────┘     └─────────────┘     └─────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Deployment Modes

| Mode | Use Case | Components |
|------|----------|------------|
| **All-in-One** | Development, small teams | Single binary: API + Worker + Scheduler |
| **Multinode** | Medium teams | Separate API and Worker containers |
| **Multinode-HA** | Production, large teams | Multiple API/Worker replicas + nginx load balancer |

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- DEPLOYMENT EXAMPLES -->
## Deployment Examples

Pre-configured Docker Compose examples are available in the [`examples/`](examples/) directory:

| Example | Description | Use Case |
|---------|-------------|----------|
| [`quickstart/`](examples/quickstart/) | Single container + dependencies | Getting started, development |
| [`multinode/`](examples/multinode/) | Separate API and worker | Small production deployments |
| [`multinode-ha/`](examples/multinode-ha/) | HA with nginx load balancer | Production with high availability |
| [`dev/`](examples/dev/) | Full dev environment with hot reload | Active development |

### Example: Multinode Deployment

```bash
cd examples/multinode
docker compose up -d
```

### Example: HA Deployment with Load Balancing

```bash
cd examples/multinode-ha
docker compose up -d --scale api=3 --scale worker=2
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONFIGURATION -->
## Configuration

Sercha Core is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `VESPA_CONFIG_URL` | Vespa config server URL | `http://localhost:19071` |
| `VESPA_CONTAINER_URL` | Vespa container URL | `http://localhost:8080` |
| `JWT_SECRET` | Secret for JWT token signing | Required |
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
- **OpenAPI Spec**: [`docs/swagger.yaml`](docs/swagger.yaml)
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
- Docker and Docker Compose
- PostgreSQL 16+
- Vespa 8+

### Local Development

```bash
# Start dependencies
docker compose -f examples/dev/docker-compose.yml up -d postgres vespa

# Run the server
go run ./cmd/sercha-core all

# Or with hot reload (requires air)
air
```

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o sercha-core ./cmd/sercha-core
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

- [Open Issues](https://github.com/custodia-labs/sercha-core/issues)
- [Pull Requests](https://github.com/custodia-labs/sercha-core/pulls)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->
## License

Distributed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[release-shield]: https://img.shields.io/github/v/release/custodia-labs/sercha-core
[release-url]: https://github.com/custodia-labs/sercha-core/releases/latest
[license-shield]: https://img.shields.io/badge/License-Apache_2.0-blue.svg
[license-url]: https://opensource.org/licenses/Apache-2.0
[goreport-shield]: https://goreportcard.com/badge/github.com/custodia-labs/sercha-core
[goreport-url]: https://goreportcard.com/report/github.com/custodia-labs/sercha-core
[ci-workflow-shield]: https://github.com/custodia-labs/sercha-core/actions/workflows/go-ci.yml/badge.svg
[ci-workflow-url]: https://github.com/custodia-labs/sercha-core/actions/workflows/go-ci.yml
[contributions-shield]: https://img.shields.io/badge/contributions-welcome-brightgreen.svg
[contributions-url]: https://github.com/custodia-labs/sercha-core/blob/main/CONTRIBUTING.md
[Go-badge]: https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white
[Go-url]: https://go.dev/
[Postgres-badge]: https://img.shields.io/badge/PostgreSQL-316192?style=flat&logo=postgresql&logoColor=white
[Postgres-url]: https://www.postgresql.org/
[Vespa-badge]: https://img.shields.io/badge/Vespa-000000?style=flat&logo=vespa&logoColor=white
[Vespa-url]: https://vespa.ai/
[Docker-badge]: https://img.shields.io/badge/Docker-2496ED?style=flat&logo=docker&logoColor=white
[Docker-url]: https://www.docker.com/
