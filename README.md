<p align="center">
  <img src=".github/assets/banner.png" alt="Sercha" width="100%">
</p>

<h4 align="center">
  Self-hosted search across all your team's tools.
</h4>

<p align="center">
  <a href="https://docs.sercha.dev/docs">Documentation</a> |
  <a href="https://discord.gg/PYagaAGf">Discord</a> |
  <a href="https://github.com/sercha-oss/sercha-core/issues/new?labels=bug&template=bug_report.md">Report Bug</a> |
  <a href="https://github.com/sercha-oss/sercha-core/issues/new?labels=enhancement&template=feature_request.md">Request Feature</a>
</p>

<p align="center">

[![GitHub Release][release-shield]][release-url]
[![License][license-shield]][license-url]
[![Go Report Card][goreport-shield]][goreport-url]
[![CI][ci-shield]][ci-url]
[![Discord][discord-shield]][discord-url]

</p>

<p align="center">
  <img src=".github/assets/hero-search-demo.gif" alt="Sercha Demo" width="100%">
</p>

Sercha Core connects your team's data sources - GitHub, Google Drive, Notion, Confluence, and more - and provides unified search across all of them. Self-hosted, so your data stays on your infrastructure.

## Features

- **Connectors** - GitHub and LocalFS today, with [12+ more planned](https://github.com/sercha-oss/sercha-core/issues?q=label%3Aconnector)
- **BM25 search** - Full-text search powered by OpenSearch
- **Semantic search** - Vector search with pgvector and configurable embedding models
- **OAuth2** - Connect data sources securely, with JWT session management
- **REST API** - Full [OpenAPI spec](https://docs.sercha.dev/api/sercha-core-api) with Swagger UI
- **Admin UI** - Web interface for managing sources, connections, and search

## Quick Start

Requires [Docker](https://docs.docker.com/get-docker/) and 4GB RAM.

```bash
git clone https://github.com/sercha-oss/sercha-core.git
cd sercha-core/examples/quickstart
docker compose --profile ui up -d
```

API available at `http://localhost:8080`, Admin UI at `http://localhost:3000`.

See the [Quickstart Guide](https://docs.sercha.dev/docs/quickstart) for the full walkthrough.

## Development

```bash
cd examples/dev
docker compose up -d --build    # API at localhost:8080

cd ui
npm install && npm run dev      # UI at localhost:3000
```

See [`examples/dev/`](examples/dev/) and [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Documentation

- [Quickstart](https://docs.sercha.dev/docs/quickstart)
- [Configuration](https://docs.sercha.dev/docs/configuration)
- [API Reference](https://docs.sercha.dev/api/sercha-core-api)
- [Connectors](https://docs.sercha.dev/connectors)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines, and [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## License

Apache 2.0 - see [LICENSE](LICENSE).

<!-- LINKS -->
[release-shield]: https://img.shields.io/github/v/release/sercha-oss/sercha-core
[release-url]: https://github.com/sercha-oss/sercha-core/releases/latest
[license-shield]: https://img.shields.io/badge/License-Apache_2.0-blue.svg
[license-url]: https://opensource.org/licenses/Apache-2.0
[goreport-shield]: https://goreportcard.com/badge/github.com/sercha-oss/sercha-core
[goreport-url]: https://goreportcard.com/report/github.com/sercha-oss/sercha-core
[ci-shield]: https://github.com/sercha-oss/sercha-core/actions/workflows/ci.yml/badge.svg
[ci-url]: https://github.com/sercha-oss/sercha-core/actions/workflows/ci.yml
[discord-shield]: https://img.shields.io/discord/1457584679669731370?label=Discord&color=5865F2
[discord-url]: https://discord.gg/PYagaAGf
