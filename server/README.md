# RCA-App Backend Server

The Backend Server is the central brain of the RCA-App. It aggregates data from node agents, maintains the live Service Map, and orchestrates health checks.

## Features

- **Service Map Builder**: Constructs a directed graph of services from network events.
- **Cycle Detection**: Identifies circular dependencies in the service graph.
- **Inspection Engine**: Internal framework to run 60+ health rules.
- **Log Clustering**: Implements a Drain-like algorithm to group log patterns.
- **Metrics Query Interface**: Unified API to query Prometheus data.
- **Enterprise Security**: RBAC (Admin/Operator/Viewer) and OIDC/JWT support.
- **Self-Monitoring**: Operational metrics exposed at `/metrics`.

## Development

### Run Locally

```bash
go mod download
go run main.go
```

The server listens on `:8080` by default.

### API Metrics

The server exposes its own metrics at `/metrics` for Prometheus scraping.

### Key Endpoints

- `GET /api/v1/servicemap`: Returns the current dependency graph.
- `GET /api/v1/inspections`: Returns health check results.
- `POST /api/v1/agent/event`: Ingestion endpoint for agents.

## Testing

Run unit tests and benchmarks:

```bash
go test ./... -bench=.
```
