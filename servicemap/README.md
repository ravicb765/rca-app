# Service Map Builder

This package implements a simple in-memory Service Map Builder used by the server.

- Build the service map from observed connections (source -> dest)
- Exposes `BuildServiceMap` and types for `Application`, `Connection` and `ServiceMap`.

Example agent event payloads that the server accepts at `POST /api/v1/agent/event`:

Single connection:

```json
{ "source_app": "frontend", "dest_app": "backend", "protocol": "http", "request_rate": 100 }
```

Multiple connections:

```json
[
  { "source_app": "frontend", "dest_app": "backend", "protocol": "http" },
  { "source_app": "backend", "dest_app": "postgres", "protocol": "tcp" }
]
```

The server returns the generated map at `GET /api/v1/servicemap` in the `servicemap` key, and preserves backwards-compatible `services` list.