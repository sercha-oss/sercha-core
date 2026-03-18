# Development Environment

Local development environment with High Availability (HA) setup for testing multi-instance configurations.

## Overview

This docker-compose configuration builds Sercha Core from source and runs a full HA setup locally:
- 2 API instances (load balanced)
- 2 Worker instances (competing for scheduler lock)
- PostgreSQL database
- Redis (distributed locking and caching)
- Vespa search engine
- nginx load balancer

## Usage

```bash
# Build and start all services
docker compose up -d --build

# View logs
docker compose logs -f

# Stop services
docker compose down

# Stop and remove volumes (full reset)
docker compose down -v
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| nginx | 8080 | API load balancer |
| sercha-api-1 | - | API instance 1 |
| sercha-api-2 | - | API instance 2 |
| sercha-worker-1 | - | Worker instance 1 |
| sercha-worker-2 | - | Worker instance 2 |
| postgres | 5432 | PostgreSQL database |
| redis | 6379 | Redis cache/queue |
| vespa | 19071 | Vespa search engine |

## When to Use

Use this environment when:
- Testing multi-instance behavior
- Debugging distributed locking
- Verifying scheduler failover
- Testing load balancer configuration

For simple development, consider using the [quickstart](../quickstart/) example instead.

## Configuration

Environment variables are set for development use only. For production configuration, see the [multinode-ha](../multinode-ha/) example.
