# Integration Test Makefile
# Handles Docker lifecycle for isolated testing

COMPOSE_DIR := ../../examples/dev
COMPOSE := docker compose -f $(COMPOSE_DIR)/docker-compose.yml

.PHONY: test test-json setup teardown wait-ready

# Run tests with full setup/teardown
test: setup wait-ready run-tests teardown

# Run tests with JSON output (for CI/AI agents)
test-json: setup wait-ready run-tests-json teardown

# Just run tests (assumes environment is ready)
run-tests:
	@cd ../.. && go test -tags=integration ./tests/integration/... -v

run-tests-json:
	@cd ../.. && GODOG_FORMAT=cucumber go test -tags=integration ./tests/integration/... -v

# Setup: rebuild and start containers fresh
setup:
	@echo "==> Setting up test environment..."
	@$(COMPOSE) down -v --remove-orphans 2>/dev/null || true
	@$(COMPOSE) up -d --build
	@echo "==> Containers started"

# Wait for API to be healthy
wait-ready:
	@echo "==> Waiting for API..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8080/health >/dev/null 2>&1; then \
			echo "==> API ready"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "==> API not ready after 60s"; \
	exit 1

# Teardown: stop and remove containers
teardown:
	@echo "==> Tearing down test environment..."
	@$(COMPOSE) down -v --remove-orphans
	@echo "==> Cleanup complete"

# Keep environment running after tests (for debugging)
test-keep: setup wait-ready run-tests
	@echo "==> Environment kept running. Run 'make teardown' when done."
