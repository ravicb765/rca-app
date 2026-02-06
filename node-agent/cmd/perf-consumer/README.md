# perf-consumer (minimal)

A small utility that reads perf events from a pinned eBPF `PERF_EVENT_ARRAY` map and forwards them as JSON POSTs to the server `/api/v1/agent/event` endpoint.

Usage:

```
go run ./cmd/perf-consumer --map /sys/fs/bpf/my_perf_map --map-name mymap --server http://localhost:8080/api/v1/agent/event
```

Flags:
- `--map` (required): Path to pinned eBPF perf map (e.g. `/sys/fs/bpf/my_perf_map`).
- `--map-name`: Logical name included in payload (default `perf_events`).
- `--server`: Destination server URL to POST events (default `http://localhost:8080/api/v1/agent/event`).
- `--token`: Optional bearer token for authentication.

Notes:
- This program is intentionally minimal and defensive. It encodes the raw perf sample bytes as base64 in the `data_base64` JSON field.
- In production you may want to decode and interpret sample payloads according to the BPF program's layout and add batching/retries and structured metadata.
