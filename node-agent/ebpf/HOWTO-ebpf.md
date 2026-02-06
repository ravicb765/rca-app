# HOWTO: eBPF debugging labs (practical)

This short HOWTO contains step-by-step exercises you can run locally to practice diagnosing common eBPF verifier issues and collecting logs for PRs.

Prerequisites
- Linux VM with kernel >= 5.4
- `clang`, `bpftool`, `libbpf-dev`, and `make` installed
- Root / sudo privileges (required to load programs)

Lab 1 — Reproduce a simple verifier "invalid read" error
1. Create a minimal C program with an out-of-bounds stack read (deliberate mistake):

```c
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

SEC("kprobe/tcp_connect")
int trace_connect(struct pt_regs *ctx) {
    char buf[1024]; // too large for verifier
    buf[1023] = 0; // likely exceed stack limits
    return 0;
}
char LICENSE[] SEC("license") = "GPL";
```

2. Compile:
```bash
clang -O2 -target bpf -c bad_stack.c -o bad_stack.o
```
3. Try to load (requires root):
```bash
sudo bpftool prog load bad_stack.o /sys/fs/bpf/bad_stack || true
```
4. Inspect kernel log:
```bash
sudo dmesg | tail -n 40
```
You should see a verifier error mentioning "stack" or "R1 invalid mem access".

Fix: Reduce stack usage (smaller arrays) or move buffers to maps.

Lab 2 — Diagnosing map size/key mismatches
1. Define a map in C with a specific key/value size:
```c
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, u32);
    __type(value, u64);
} my_map SEC(".maps");
```
2. In your loader (Go), ensure you access the map with matching types. Use `bpftool map show` to inspect the map definition after load.

Lab 3 — Verifier helper failures
- If using helpers like `bpf_probe_read`, confirm your kernel supports them; otherwise the verifier will reject calls. Check kernel version and BTF availability.

Collecting logs for PRs
- In CI (see `.github/workflows/ebpf-ci.yml`) we collect `dmesg`, `bpftool prog show`, `bpftool map show` and `/sys/kernel/debug/tracing/trace` (if present) and upload as artifacts named `ebpf-logs`.
- Locally, capture the same and paste relevant snippets into your PR.

Good practices
- Keep C changes small and iterative; run syntax-only checks in CI (`clang -fsyntax-only`).
- Include `.o` artifacts or rely on ebpf CI to upload compiled objects and logs for maintainers to inspect.
- When you see a verifier error, include the exact `dmesg` lines and describe the kernel version and commands you ran in your PR.

Fixed vs broken — sample outputs (what to look for)

1) `bad_stack.c` (broken)
- Compile log may be empty (C compiles), but loading usually shows verifier output in `dmesg` like:
```
[ 1234.567890] BPF: Verifier rejected program: R1 invalid mem access 'read'  'stack'
[ 1234.567891] BPF: R1=r2 off=0 imm=0 ???
[ 1234.567892] BPF: stack depth 528 is too large
```
- Action: reduce local arrays, move large data to maps, or split logic.

2) `fixed_stack.c` (fixed)
- Compile log: clean; loading (on test VM) should not show stack-related verifier errors.
- Expected compile output snippet (no warnings): `Compiled fixed_stack.c cleanly`

3) `bad_map_key.c` (broken)
- Compiler may succeed but `bpftool` or verifier may complain about incorrect map access patterns at load time, or runtime lookups will fail.
- Look for `bpftool map show` to verify map key/value sizes and reconcile with C and loader code.

4) `fixed_map_key.c` (fixed)
- Compile log: clean; map types align with loader expectations.

CI compile logs
- The `ebpf-examples` workflow (PRs touching `node-agent/ebpf/**`) runs `compile_and_log_examples.sh` and uploads `logs/*.compile.log` as artifact `ebpf-example-compile-logs` so maintainers can inspect compiler warnings/errors without requiring kernel load privileges.

Example `dmesg` snippet to include in PRs:
```
[ 1234.567890] BPF: Verifier rejected program: R1 invalid mem access 'read'  'stack'
[ 1234.567891] BPF: R1=r2 off=0 imm=0 ???
```

If you'd like, I can add a small `examples/` folder under `node-agent/ebpf/` containing intentionally bad programs for practice and a test harness to run them safely in a VM. Say the word and I'll add it.