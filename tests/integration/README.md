# Integration Tests

BDD-style integration tests using [godog](https://github.com/cucumber/godog) (Cucumber for Go).

These tests use a `//go:build integration` tag, so they're **excluded from `go test ./...`** by default. This prevents CI failures when Docker isn't available.

## Quick Start

```bash
# Run all integration tests (handles Docker lifecycle)
cd tests/integration
make test

# Run tests but keep environment running (for debugging)
make test-keep

# Just run tests (if environment already running)
make run-tests

# Teardown environment
make teardown
```

## Structure

```
tests/integration/
├── Makefile              # Docker lifecycle & test commands
├── integration_test.go   # Test entrypoint
├── features/             # Gherkin feature files
│   ├── ftue.feature      # First-time user experience
│   └── localfs.feature   # LocalFS connector tests
├── steps/
│   └── steps.go          # Step definitions
└── support/
    └── context.go        # Test context & utilities
```

## Writing Features

Features use [Gherkin syntax](https://cucumber.io/docs/gherkin/reference/):

```gherkin
Feature: LocalFS Source Indexing
  As an administrator
  I want to index local files
  So they are searchable

  Background:
    Given I am logged in as admin

  Scenario: Index local directory and search
    When I create a localfs installation with path "/data/test-docs"
    Then the response status should be 201
    And I should have an installation ID

    When I search for "README"
    Then I should see search results
```

### Guidelines

1. **One feature per file** - Name files after the feature being tested
2. **Use Background** for common setup (login, wait for services)
3. **Keep scenarios focused** - Test one behavior per scenario
4. **Use descriptive step names** - They become documentation

## Adding New Steps

Steps are defined in `steps/steps.go`:

```go
func InitializeScenario(ctx *godog.ScenarioContext) {
    tc := &TestContext{}

    // Given steps
    ctx.Step(`^I am logged in as admin$`, tc.iAmLoggedInAsAdmin)

    // When steps
    ctx.Step(`^I create a localfs installation with path "([^"]*)"$`, tc.iCreateLocalfsInstallation)

    // Then steps
    ctx.Step(`^the response status should be (\d+)$`, tc.theResponseStatusShouldBe)
}

func (tc *TestContext) iCreateLocalfsInstallation(path string) error {
    // Implementation
    resp, err := tc.client.Post("/api/v1/admin/installations", map[string]any{
        "name": "Test Install",
        "provider_type": "localfs",
        "api_key": path,
    })
    tc.lastResponse = resp
    return err
}
```

### Step Patterns

| Pattern | Matches |
|---------|---------|
| `^literal text$` | Exact match |
| `"([^"]*)"` | Quoted string |
| `(\d+)` | Integer |
| `(true\|false)` | Boolean |

## Test Context

The `TestContext` struct in `support/context.go` holds:

- HTTP client configured for test API
- Last response (for assertions)
- Created resource IDs (for cleanup)
- Helper methods

```go
type TestContext struct {
    client       *http.Client
    baseURL      string
    authToken    string
    lastResponse *http.Response
    lastBody     []byte

    // Resource tracking
    installationID string
    sourceID       string
}
```

## Environment

Tests run against the dev Docker environment:

| Service | Port | Purpose |
|---------|------|---------|
| API | 8080 | Sercha Core API |
| PostgreSQL | 5432 | Database with pgvector |
| OpenSearch | 9200 | BM25 text search |

The Makefile handles starting/stopping these containers.

## CI Integration

Tests run in CI via GitHub Actions:

```yaml
- name: Run Integration Tests
  run: |
    cd tests/integration
    make test-json
```

The `test-json` target outputs Cucumber JSON format for CI reporting.

## Debugging

### Keep environment running

```bash
make test-keep
# Tests run, environment stays up
# Inspect containers, check logs, etc.
make teardown  # When done
```

### View container logs

```bash
cd examples/dev
docker compose logs -f sercha
docker compose logs -f opensearch
```

### Run single scenario

```bash
cd ../..
go test ./tests/integration/... -v -godog.tags="@wip"
```

Add `@wip` tag to scenario:
```gherkin
@wip
Scenario: My scenario under development
```

## Best Practices

1. **Test critical paths** - FTUE, sync, search, auth
2. **Don't duplicate unit tests** - Integration tests are for flows
3. **Keep scenarios independent** - Each should run in isolation
4. **Clean up resources** - Use AfterScenario hooks
5. **Wait for async operations** - Use polling, not sleep
6. **Use meaningful assertions** - Check specific values, not just status codes
