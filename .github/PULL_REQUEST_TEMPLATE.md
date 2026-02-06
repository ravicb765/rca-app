## Quick PR checklist
- [ ] Tests: `go test ./...` and `pytest` pass locally (or CI passes).
- [ ] Docs: update `README.md`, `BACKSTAGE-INTEGREATION.md`, or relevant docs for behavioral/config changes.
- [ ] UI: include screenshots and confirm `yarn --cwd packages/app build` completes for frontend changes.
- [ ] eBPF: run `make` or `./node-agent/ebpf/compile-ebpf.sh`; attach compiled `.o` objects or ensure CI artifacts contain them. Note verifier/load checks and runner type in the PR description.

Suggested labels: `area/node-agent`, `area/server`, `area/ml`, `area/ui`, `eBPF`, `docs`, `tests`, `breaking-change` (add the most relevant ones).

Reviewers: changes will auto-request the code owners; for eBPF or node-agent changes also request review from an eBPF maintainer (e.g., @ravicb765) if not already assigned.

Please ensure your PR follows the QA checklist in `.github/copilot-instructions.md` (tests, docs, screenshots for UI changes, and eBPF safety checks where applicable).