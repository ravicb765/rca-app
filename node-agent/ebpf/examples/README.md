# eBPF Examples (Lab Programs)

This folder contains intentionally failing or edge-case eBPF programs for local testing and debugging practice. **Do not run these files on shared or production hosts.** Use an isolated VM with a development kernel and sudo privileges.

Files:
- `bad_stack.c` - demonstrates excessive stack usage that should trigger a verifier "stack" error
- `bad_map_key.c` - demonstrates a map/key mismatch scenario
- `compile_examples.sh` - convenience script to compile the examples to `.o` objects (does not load them into kernel)

Usage:
1. From repo root: `cd node-agent/ebpf/examples`
2. Compile broken examples to see errors: `./compile_examples.sh` (requires `clang`).
3. Compile both broken and fixed examples and capture logs: `./compile_and_log_examples.sh` — this writes `logs/*.compile.log` with compiler output for each example.
4. Inspect compiler output and check for expected errors. If you want to test loading, follow the eBPF CI guidance and run on an isolated privileged VM.

Fixed examples
- `fixed_stack.c` — safe reduced stack usage
- `fixed_map_key.c` — correct key type usage for map operations

Expected outcomes
- `bad_stack.c` -> compiler may succeed but loading will likely produce verifier errors; check `dmesg` and verifier logs.
- `fixed_stack.c` -> should compile cleanly and not produce verifier stack complaints.
- `bad_map_key.c` -> may compile but map access is incorrect; the fixed version uses `u64` keys and updates/looks up correctly.

CI behavior
- On PRs that modify `node-agent/ebpf/**`, the repository will automatically run `compile_and_log_examples.sh` and upload the compile logs as an artifact named `ebpf-example-compile-logs` (see `.github/workflows/ebpf-examples.yml`).  This provides quick compiler output for maintainers without loading programs into the kernel.

Safety:
- These examples are for teaching diagnostics only. They may trigger kernel verifier logs when loaded; capture `dmesg` output and include it in PRs as described in the project howto/HOWTO-ebpf.md.