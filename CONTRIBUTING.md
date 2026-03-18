# Contributing to Sercha Core

Thank you for your interest in contributing to Sercha Core! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Branch Naming](#branch-naming)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Running CI Locally](#running-ci-locally)
- [Testing](#testing)
- [UI Development](#ui-development)
- [API Changes](#api-changes)
- [Governance](#governance)

## Getting Started

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/sercha-core.git
   cd sercha-core
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/custodia-labs/sercha-core.git
   ```
4. Keep your fork synced:
   ```bash
   git fetch upstream
   git checkout main
   git merge upstream/main
   ```

### Prerequisites

- Go 1.24 or later
- Docker and Docker Compose
- Node.js 20+ (for UI development)
- golangci-lint (for linting)
- swag (for Swagger generation)

### Install Development Tools

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install swag for Swagger generation
go install github.com/swaggo/swag/cmd/swag@latest
```

## Project Structure

```
sercha-core/
├── cmd/
│   └── sercha-core/
│       └── main.go              # Application entry point
├── internal/                     # Private application code
│   ├── adapters/                # Hexagonal architecture adapters
│   │   ├── driven/              # Infrastructure (storage, search, connectors)
│   │   │   ├── connectors/      # Data source connectors (GitHub, etc.)
│   │   │   ├── postgres/        # Database implementation
│   │   │   └── vespa/           # Search engine implementation
│   │   └── driving/             # Entry points (HTTP handlers)
│   │       └── http/            # REST API handlers
│   └── core/                    # Business logic
│       ├── domain/              # Domain models
│       ├── ports/               # Interface definitions
│       └── services/            # Service implementations
├── ui/                          # Next.js Admin UI
│   ├── src/
│   │   ├── app/                 # Next.js app router pages
│   │   ├── components/          # React components
│   │   └── lib/                 # Utilities and API client
│   └── Dockerfile               # UI build configuration
├── examples/                    # Deployment examples
│   ├── quickstart/              # Single container setup
│   ├── multinode/               # Separate API/worker
│   ├── multinode-ha/            # High availability setup
│   └── dev/                     # Development environment
├── docs/                        # Documentation and assets
├── .github/
│   └── workflows/               # GitHub Actions
├── go.mod                       # Go module definition
└── README.md
```

## Development Workflow

### Setting Up the Development Environment

```bash
# Start dependencies (PostgreSQL, Vespa)
cd examples/dev
docker compose up -d postgres vespa

# Wait for Vespa to initialize (1-2 minutes)
docker compose logs -f vespa

# Run the server in development mode
cd ../..
go run ./cmd/sercha-core all
```

### Daily Development

```bash
# Start work
git checkout main
git pull upstream main
git checkout -b feat/my-feature

# Make changes
# ...edit files...

# Run checks
go mod tidy
go vet ./...
golangci-lint run
go test ./...

# Commit
git add .
git commit -m "feat(api): add new endpoint"

# Push and create PR
git push origin feat/my-feature
```

**All code changes must go through pull requests and pass CI.**

## Branch Naming

Use the following pattern for all branches:

```
type/short-description
```

### Branch Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat/add-gitlab-connector` |
| `fix` | Bug fix | `fix/oauth-token-refresh` |
| `docs` | Documentation | `docs/api-reference` |
| `style` | Code style/formatting | `style/format-handlers` |
| `refactor` | Code refactoring | `refactor/extract-search-service` |
| `perf` | Performance improvement | `perf/optimize-batch-indexing` |
| `test` | Tests | `test/add-integration-tests` |
| `chore` | Maintenance | `chore/update-dependencies` |

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
type(scope): summary

[optional body]

[optional footer(s)]
```

### Scope

The scope should be the module or area affected:

- `api` - REST API handlers
- `search` - Search functionality
- `connectors` - Data source connectors
- `auth` - Authentication/authorization
- `worker` - Background task processing
- `ui` - Admin UI
- `deps` - Dependencies

### Examples

```bash
# Simple commit
feat(connectors): add GitLab repository connector

# With body
fix(auth): correct JWT token expiration handling

The token expiration was being calculated incorrectly when
the system clock was out of sync. Now uses server time.

Fixes #123

# Breaking change
feat(api)!: change search response format

BREAKING CHANGE: The search response now includes pagination
metadata in a separate object instead of flat fields.
```

### Rules

1. **Use imperative mood** - "add" not "added" or "adds"
2. **Don't capitalize** - "add feature" not "Add feature"
3. **No period at end** - "add feature" not "add feature."
4. **Keep summary under 72 characters**
5. **Reference issues** when applicable

## Pull Requests

### Before Opening a PR

1. **Sync with main**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks**
   ```bash
   go mod tidy
   go vet ./...
   golangci-lint run
   go test ./...
   go build ./...
   ```

3. **Update Swagger docs** (if API changed)
   ```bash
   swag init -g cmd/sercha-core/main.go -o docs
   ```

4. **Ensure clean commits** - Squash WIP commits if needed

### PR Requirements

| Requirement | Description |
|-------------|-------------|
| CI Passing | All GitHub Actions checks must be green |
| Review | At least one approving review required |
| Up to Date | Branch must be current with `main` |
| Description | Clear explanation of changes |
| Tests | New functionality should include tests |

### Review Process

1. Open PR with clear description
2. Wait for CI checks to pass
3. Request review from maintainers
4. Address feedback with additional commits
5. Once approved, maintainer will merge

## Running CI Locally

Before submitting a PR, run the same checks that CI runs:

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run vet
go vet ./...

# Run linter
golangci-lint run

# Tidy modules
go mod tidy

# Check for uncommitted changes (modules should be tidy)
git diff --exit-code go.mod go.sum
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/core/services/...

# Run integration tests (requires Docker)
go test -tags=integration ./...
```

### Writing Tests

- Place tests in `_test.go` files alongside the code
- Use table-driven tests where appropriate
- Mock external dependencies using interfaces
- Aim for meaningful test coverage on business logic

### Test Database

For tests that require a database:

```bash
# Start test PostgreSQL
docker run -d --name postgres-test \
  -e POSTGRES_USER=test \
  -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=sercha_test \
  -p 5433:5432 \
  postgres:16-alpine

# Set test database URL
export DATABASE_URL="postgres://test:test@localhost:5433/sercha_test?sslmode=disable"
```

## UI Development

The Admin UI is a Next.js application in the `ui/` directory.

### Setup

```bash
cd ui
npm install
```

### Development Server

```bash
# Start the UI dev server (connects to local API)
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev

# UI available at http://localhost:3000
```

### Building

```bash
# Build static export
npm run build

# Output in ui/out/
```

### UI Guidelines

- Use TypeScript for all new code
- Follow existing component patterns
- Use Tailwind CSS for styling
- Ensure accessibility (ARIA labels, keyboard navigation)

## API Changes

When modifying the REST API:

1. **Update handler code** in `internal/adapters/driving/http/handlers.go`

2. **Update Swagger annotations** - Add/modify swagger comments:
   ```go
   // @Summary Create a new source
   // @Tags sources
   // @Accept json
   // @Produce json
   // @Param source body CreateSourceRequest true "Source to create"
   // @Success 201 {object} Source
   // @Router /api/v1/sources [post]
   ```

3. **Regenerate Swagger docs**:
   ```bash
   swag init -g cmd/sercha-core/main.go -o docs
   ```

4. **Update UI API client** in `ui/src/lib/api.ts` if needed

5. **Add tests** for new endpoints

## Governance

### Roles

**Maintainers** have write access and are responsible for:
- Reviewing and merging pull requests
- Triaging issues
- Ensuring code quality
- Helping contributors

**Contributors** participate through:
- Code contributions
- Documentation improvements
- Bug reports and feature requests
- Helping other users

### Decision Making

- **Routine decisions** (bug fixes, minor improvements) are made by individual maintainers
- **Significant decisions** (new features, breaking changes) require discussion and maintainer consensus
- **Disputes** are resolved through discussion; project owner makes final decision if needed

### Becoming a Maintainer

Contributors may be invited based on:
- Consistent, high-quality contributions
- Understanding of project goals and conventions
- Positive interactions with the community

## Questions?

If you have questions, please open an issue or reach out to the maintainers.

See also:
- [Security Policy](SECURITY.md)
- [License](LICENSE)
