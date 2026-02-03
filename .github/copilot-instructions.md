# Copilot / AI Agent Instructions for RCA-App 🔧

## Quick Orientation (2 sentences) 💡
- RCA-App is an observability platform: eBPF-based `node-agent` collects telemetry → `server` (Go/Gin) aggregates and exposes APIs → `ml-service` (Python/Flask) performs RCA → `backstage-portal` provides the UI.
- Key entry points: `README.md`, `PROJECT-SUMMARY.md`, `BACKSTAGE-INTEGREATION.md`, `docker-compose.yml` and the component roots: `node-agent/`, `server/`, `ml-service/`, `backstage-portal/`.

---

## What to do first (startup checks) ✅
- Start local dev: `docker-compose up -d` (see `docker-compose.yml`). Use `docker-compose logs -f` to inspect service startup.
- Validate APIs quickly:
  - ML: POST `http://localhost:5000/analyze` (example payload in `README.md`).
  - Server: GET `http://localhost:8080/api/v1/servicemap` and `.../applications`.
- eBPF work requires Linux with kernel >= 5.4 and often `sudo` to run the agent (see `README.md` and quick reference).

---

## Repo-specific conventions & patterns 🔍
- Back-end services are split into small component folders: `node-agent` (Go + eBPF), `server` (Go/Gin), `ml-service` (Python/Flask). New components should follow the same language / framework for consistency.
- Backstage plugins live under `backstage-portal/plugins/` — naming convention: `@rca-app/plugin-*` (see `BACKSTAGE-INTEGREATION.md`).
- Docker images are defined per-component with `Dockerfile` in each top-level service folder; prefer local `docker-compose` entries for development.
- Storage integrations: ClickHouse (logs/traces), Prometheus (metrics), Redis (cache). Look to `README.md` for ClickHouse schema examples and retention notes.

---

## Testing & CI tips 🧪
- Unit tests:
  - Go: `go test ./...`
  - Python: `pytest tests/` (run inside `ml-service` virtualenv)
- Integration tests: `docker-compose -f docker-compose.test.yml up -d` then run the repo's integration test script (docs reference `./scripts/run-integration-tests.sh`—confirm presence before use).
- When adding tests, prefer small, deterministic units that don't require eBPF or kernel features; mock network/eBPF inputs when possible.

---

## Common tasks & concrete examples 🛠️
- Build and run server locally:
  - `cd server && go build -o server main.go && ./server`
- Run ML service locally:
  - `cd ml-service && python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt && python main.py`
- Debugging flow: start `docker-compose`, tail `docker-compose logs -f`, hit API endpoints from host and check `clickhouse` / `prometheus` metrics.

---

## Integration points & APIs to pay attention to 🔗
- `ml-service` exposes `/analyze` (example in `README.md`) — used by `server` to fetch RCA results.
- `server` exposes REST endpoints under `/api/v1/` for the UI (service map, applications). Search for these paths in `server/` when making changes.
- `node-agent` forwards telemetry to the `server` (look for configuration env `RCA_APP_ENDPOINT` in docs).

---

## eBPF Development Tips 🐛
- Primary locations:
  - BPF C sources: `node-agent/ebpf/` (README shows `network_tracer.c` as an example).
  - Loader & map interactions: `node-agent/main.go` and other Go files that use `github.com/cilium/ebpf`.
  - Docker and deployment: `node-agent/Dockerfile` and `docker-compose.yml` (DaemonSet example in `README.md`).
- Typical workflow:
  - Edit C source -> compile to object:
    `clang -O2 -target bpf -c node-agent/ebpf/network_tracer.c -o node-agent/ebpf/network_tracer.o`
  - Build the Go binary that loads the object (see `node-agent/main.go` for `ebpf` collection loading).
  - Run locally on a Linux VM (kernel >= 5.4) with sudo: `sudo ./node-agent` or `sudo go run main.go`.
- Debugging & verification:
  - Use `bpftool prog show` / `bpftool map show` to inspect loaded programs and maps.
  - Check runtime traces: `sudo cat /sys/kernel/debug/tracing/trace` and `dmesg | tail` for verifier errors.
  - If a program fails the verifier, fix C code incrementally and recompile; check the kernel log for specific errors.
- Safety & best practices:
  - Make small, incremental C changes; avoid complex loops, large stacks, or unbounded memory.
  - Prefer unit tests and Go-level mocks for logic outside BPF programs; eBPF-specific logic should be exercised in an isolated VM.
  - Run eBPF changes in a non-production VM or a privileged CI job before merging.
  - Ensure Kubernetes DaemonSet uses `securityContext.privileged: true` for node-agent when testing in-cluster (example in `README.md`).
- Tooling:
  - Common tools: `clang` (BPF object), `bpftool` (from iproute2), `bpftool prog` / `bpftool map`.
  - Confirm `clang --version` and `bpftool` are available on your dev machine or test VM.

- CI Example (GitHub Actions):
  - Purpose: compile eBPF C sources to BPF objects and optionally run verifier/load checks on a privileged self-hosted runner.
  - Minimal workflow snippet (place in `.github/workflows/ebpf-ci.yml`):

```yaml
name: eBPF CI
on:
  push:
    paths:
      - 'node-agent/ebpf/**'
  pull_request:
    paths:
      - 'node-agent/ebpf/**'

jobs:
  build-objects:
    # Requires a self-hosted runner with an `ebpf` label and access to install packages and run verifier steps
    runs-on: [self-hosted, linux, x64, ebpf]
    steps:
      - uses: actions/checkout@v4

      - name: Install build deps
        run: |
          sudo apt-get update
          sudo apt-get install -y clang llvm libbpf-dev bpftool make

      - name: Compile BPF objects
        run: |
          for f in node-agent/ebpf/*.c; do
            echo "Compiling $f"
            clang -O2 -target bpf -c "$f" -o "${f%.c}.o"
          done

      - name: List compiled objects
        run: ls -la node-agent/ebpf/*.o

      - name: Optional verifier/load (requires privileged runner)
        run: |
          # Loading into the kernel requires privileged access; this step is optional and may fail on hosted runners
          set -e
          sudo bpftool prog load node-agent/ebpf/network_tracer.o /sys/fs/bpf/network_tracer || true
          sudo bpftool prog show || true

      - name: Upload compiled objects
        uses: actions/upload-artifact@v4
        with:
          name: ebpf-objects
          path: node-agent/ebpf/*.o
```

  - Notes:
    - GitHub-hosted runners do NOT allow privileged kernel operations; run the verifier/load steps only on a self-hosted runner with appropriate permissions.
    - At minimum, ensure the C files compile (`clang -fsyntax-only`) on CI to catch syntax/regression bugs in PRs.

---

## PR reviewer QA checklist ✅
- Tests: Unit tests updated/added; `go test ./...` and `pytest` pass locally.
- Docs: Update `README.md`, `BACKSTAGE-INTEGREATION.md`, or plugin docs when behavior/config changes; include curl/snippet examples for API changes.
- UI: For Backstage changes include screenshots and confirm `yarn --cwd packages/app build` completes without errors.
- eBPF Safety (for `node-agent/ebpf/*` changes):
  - Confirm the C compiles to a BPF object via `clang -O2 -target bpf -c`.
  - Verify programs load and appear in `bpftool prog show` on a test Linux VM (kernel >= 5.4).
  - Ensure no verifier errors in `dmesg` and no unsafe syscalls or unbounded memory usage.
- CI & Docker: Confirm Dockerfiles build and `docker-compose up -d` brings services up in local dev.
- Security: No secrets checked-in; TLS and RBAC changes are documented and reviewed.
- Scope & Size: Keep PRs focused; split large changes into follow-ups when possible.

---

## Safe assumptions vs. check-before-change ⚠️
- Assume `docker-compose` is the primary local dev workflow unless a CHANGELOG/CI says otherwise.
- Do not change kernel-eBPF code without a local eBPF test harness; these require privileged execution and specific kernel support.
- Some scripts/refs in docs (e.g., `scripts/run-integration-tests.sh`) may be placeholders—verify existence before running.

---

## Helpful search terms for agents 🔎
- `main.go`, `main.py`, `Dockerfile`, `/analyze`, `/api/v1/servicemap`, `backstage-portal/plugins`, `cilium/ebpf`, `clickhouse`.

---

## Pull request guidance 👇
- Keep changes small and focused (1 feature/bug per PR).
- Add or update unit tests and a short integration test (if applicable).
- For Backstage changes, include example UI screenshots and the `yarn` build step used: `yarn --cwd packages/app build`.

---

If anything in this guidance is unclear or you want more detail for a specific component (e.g., eBPF program structure or ML model training steps), tell me which area and I will expand the instructions. ✅