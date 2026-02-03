# Label guidelines

This document maps repository labels to examples and recommended usage. Use these labels to help triage and route PRs and issues quickly.

Core labels and usage

- area-node-agent
  - Purpose: Changes related to the `node-agent` and eBPF sources.
  - Examples: adding an eBPF program, changing the agent loader, node-agent Dockerfile changes.
  - Use when: PR touches files under `node-agent/`.

- area-server
  - Purpose: Backend server changes.
  - Examples: new API endpoints under `server/`, service map changes, inspections.
  - Use when: PR touches files under `server/`.

- area-ml
  - Purpose: ML / AI service changes.
  - Examples: model updates, `/analyze` API changes, training data updates.
  - Use when: PR touches files under `ml-service/`.

- area-ui
  - Purpose: Backstage portal and plugin changes.
  - Examples: new plugin, UI layout changes, plugin backend routes.
  - Use when: PR touches files under `backstage-portal/`.

- eBPF
  - Purpose: Kernel-level or eBPF-specific changes requiring extra review and safety checks.
  - Examples: changes to `node-agent/ebpf/*.c`, BPF map definition changes.
  - Use when: any change in `node-agent/ebpf/`.
  - Notes: eBPF PRs should include compiled `.o` artifacts or ensure CI artifacts include them and document any verifier/dmesg output.

Supporting labels

- docs — documentation changes or additions.
- tests — adding/modifying tests.
- breaking-change — indicates potential backward-incompatible changes.
- urgent / urgent-review — mark issues or PRs that need expedited attention.

Labeling etiquette

- Add one or two `area-*` labels that best describe the primary scope.
- Use `eBPF` for any kernel/BPF source changes in addition to `area-node-agent`.
- Add `docs` and `tests` where relevant (e.g., if a PR updates docs or adds tests).
- Use `urgent` / `urgent-review` only for time-sensitive issues and include context in the PR description.

If you're unsure which labels to add, add a comment asking maintainers to suggest labels or rely on auto-assigned CODEOWNERS reviewers to help triage.