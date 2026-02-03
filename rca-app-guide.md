# RCA-App Guide — Developer & Maintainer Reference

This guide consolidates actionable developer steps, eBPF workflows, CI/testing notes, and project conventions for RCA-App.

## Quick developer setup (minutes)
- Clone repo: `git clone https://github.com/ravicb765/rca-app && cd rca-app`
- Start local dev stack (server + ml + node-agent skeleton): `docker compose up -d`
- Run simple checks:
  - Server: `curl http://localhost:8080/api/v1/servicemap`
  - ML: POST `http://localhost:5000/analyze` (see `README.md` examples)

## eBPF development (practical steps)
- Files:
  - C sources: `node-agent/ebpf/*.c`
  - Loader & Go interaction: `node-agent/loader_ebpf.go` (enabled with build tag `ebpf`)
  - Compile helper: `node-agent/ebpf/Makefile` and `node-agent/ebpf/compile-ebpf.sh`
- Local workflow:
  1. Compile objects: `cd node-agent/ebpf && make` or `./compile-ebpf.sh`
  2. Build node-agent with loader: `cd node-agent && go get github.com/cilium/ebpf@latest && go build -tags ebpf -o node-agent .`
  3. Run (requires sudo): `sudo ./node-agent` or `sudo go run -tags ebpf main.go`
- Debugging tips:
  - Inspect programs/maps: `sudo bpftool prog show && sudo bpftool map show`
  - Kernel output: `dmesg | tail` and `sudo cat /sys/kernel/debug/tracing/trace`
- CI note: eBPF load/attach checks require a privileged self-hosted runner; GitHub-hosted runners cannot load kernel programs. See `.github/workflows/ebpf-ci.yml`.

## Tests & CI (what runs where)
- Unit tests:
  - Go: `go test ./...` (server tests in `server/main_test.go`)
  - Python: `pytest` (ML tests in `ml-service/tests/`)
- Integration tests:
  - Local: `./scripts/run-integration-tests.sh` (uses `docker-compose.test.yml` to build & run server + ml-service, runs simple health checks and POST to `/analyze`).
  - CI: `.github/workflows/integration.yml` runs the integration script on PRs touching server or ml-service.
- Smoke checks:
  - `.github/workflows/ci.yml` runs unit tests and an eBPF syntax check (`clang -fsyntax-only`).

## Coding & PR conventions (practical)
- Keep PRs small and focused (1 change/feature per PR).
- For eBPF changes: compile C locally and attach `.o` artifacts to PR or ensure the ebpf CI artifacts include them.
- Add tests for behavior changes; update docs when introducing new configuration.
- Include screenshots for Backstage/UI changes and confirm `yarn --cwd packages/app build` succeeds.
- Use labels: `area-node-agent` / `area-server` / `area-ml` / `area-ui` plus `eBPF`, `docs`, `tests` as appropriate.

## Where to look next (key files to inspect)
- `node-agent/ebpf/*` — eBPF C sources, Makefile, compile helper
- `node-agent/loader_ebpf.go` & `node-agent/loader_stub.go` — loader implementation and stub
- `server/` — Gin-based API and tests
- `ml-service/` — Flask-based ML API and tests
- `.github/workflows/` — CI, eBPF CI, integration and smoke workflows
- `scripts/run-integration-tests.sh` — integration harness for CI/local

## Troubleshooting common issues
- Docker build fails in CI with missing `go.sum`: run `go mod tidy` locally, confirm `server/Dockerfile` runs `go mod tidy` before `go build`.
- eBPF verifier errors: iterate on C locally, check `dmesg`, and test on an isolated VM with matching kernel and headers.

## eBPF troubleshooting cookbook (practical examples)
When working with eBPF, the kernel verifier can be strict — here are common errors and how to inspect and fix them.

1) "R1 invalid mem access" / "invalid read from stack"
- What it means: your program accessed memory out of bounds (stack or map) or used an incorrect pointer offset.
- Inspect: `sudo dmesg | tail -n 50` will show verifier messages with the failing instruction and map/stack access details.
- Fixes:
  - Use bounded reads/writes and validate pointer arithmetic.
  - Keep stack usage small (move large buffers to maps or BPF per-cpu arrays).

Example dmesg snippet:
```
BPF verifier or helper is complaining: R1 invalid mem access 'read'  'stack'.
program '' could not be loaded: -EFAULT
```

2) "exceeded stack limit" / "stack frame too large"
- What it means: eBPF stack is limited (often 512 bytes); large local arrays or deep recursion cause failures.
- Fixes:
  - Reduce local stack usage; move larger data into BPF maps or ring buffers.
  - Break complex logic into simpler helpers or offload processing to userspace.

3) "loop unrolled too many times" / "unbounded loop"
- What it means: verifier doesn't allow arbitrary loops; bounded loops must be provably finite.
- Fixes:
  - Use fixed-size loops with small, constant bounds.
  - Prefer per-event processing instead of unbounded loops.

4) "invalid map type or key" / "map lookup failed"
- What it means: using a map with wrong key/value sizes or the map wasn't found/created.
- Inspect: `bpftool map show` and ensure map defs match between C and userspace loader.
- Fixes:
  - Align C map definitions with loader expectations; use explicit `__uint(key_size, ...)` fields.

5) Verifier helper errors (unsupported helper or wrong call signature)
- What it means: using a helper that isn't available or passing wrong argument types.
- Fixes:
  - Check helper availability for your kernel (helpers differ across kernel versions).
  - Use helpers with correct prototype and types.

General debugging commands
- List programs & maps: `sudo bpftool prog show && sudo bpftool map show`
- Dump verifier log (if loader supports it): `bpftool prog loadall ... verbose` or check `dmesg` for verifier output.
- Inspect BTF / kernel headers: ensure `linux-headers` and `libbpf` tooling are installed on your dev VM.

Best practices
- Make small, incremental C edits and rebuild frequently.
- Add automated syntax checks to CI (`clang -fsyntax-only`) and upload `.o` artifacts for maintainers' inspection.
- Test on an isolated VM with the same kernel version as your target environment before merging.

---

If you'd like, I can expand this cookbook with real `dmesg` outputs from current `node-agent/ebpf/` sources and a small `HOWTO-ebpf.md` with step-by-step debugging exercises. Tell me which you'd prefer.