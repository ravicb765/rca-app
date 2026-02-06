# eBPF development (node-agent)

This file documents common local development, build and debug commands for eBPF sources used by `node-agent`.

Prerequisites
- Linux kernel >= 5.4 (eBPF support)
- `clang`, `llvm`, `bpftool`, and `make` installed
- Root or sudo privileges to load programs and access kernel tracing

Quick compile
```bash
# Compile a C source to a BPF object
clang -O2 -target bpf -c node-agent/ebpf/network_tracer.c -o node-agent/ebpf/network_tracer.o

# Verify syntax only on CI / non-privileged runners
clang -fsyntax-only node-agent/ebpf/network_tracer.c
```

Build & run loader
```bash
# Build Go binary that loads the .o (loader in node-agent/main.go)
cd node-agent
go build -o node-agent main.go

# Run locally (requires sudo for eBPF attach/load)
sudo ./node-agent
# or for quick iteration
sudo go run main.go
```

Debug & verification
```bash
# Check loaded programs and maps
sudo bpftool prog show
sudo bpftool map show

# Kernel verifier / runtime errors
sudo dmesg | tail -n 50
sudo cat /sys/kernel/debug/tracing/trace | tail -n 100

# Use bpftool to dump map contents (if map type supports it)
sudo bpftool map dump id <map-id>
```

CI notes
- Use a self-hosted privileged runner for verifier/load checks (GitHub-hosted runners cannot load programs into the kernel).
- At minimum, ensure the C sources compile in CI (`clang -fsyntax-only`) and upload `.o` artifacts for manual inspection.
- Example workflow exists at `.github/workflows/ebpf-ci.yml`.

Local developer workflow (how to run the loader locally)

1. Compile eBPF objects

```bash
# from repo root
cd node-agent/ebpf
make
# or
./compile-ebpf.sh
```

2. Build the node-agent with the eBPF loader enabled

```bash
# install go deps if needed
cd node-agent
# ensure github.com/cilium/ebpf is available
go get github.com/cilium/ebpf@latest
# build with the 'ebpf' tag (enables loader_ebpf.go)
go build -tags ebpf -o node-agent .
```

3. Run the agent (requires root to load/attach programs)

```bash
sudo ./node-agent
# or run using go run (also needs sudo privileges)
sudo go run -tags ebpf main.go
```

Notes:
- The `loader_ebpf.go` file attempts to load `node-agent/ebpf/network_tracer.o` using `github.com/cilium/ebpf`.
- Do development and verifier checks inside an isolated VM or a dedicated test host (do not run untrusted eBPF code on shared runners).

Safety & best practices
- Make small, focused C changes; avoid large stacks and complex loops.
- Mock and unit test non-eBPF logic in Go; exercise eBPF programs in an isolated VM before merging.
- Do not attempt kernel loads on shared or production runners without explicit approvals.

Contact
- If unsure about a verifier error or required privileges, add a note to your PR and request a review from a maintainer experienced with eBPF.