# Contributing to RCA-App

Thanks for your interest! Below are quick rules and expectations to help contributions get merged quickly.

## General PR Guidelines
- Keep PRs small and focused (1 feature/bug per PR).
- Add or update unit tests and ensure `go test ./...` and `pytest` pass locally.
- Include a short description and reproducible steps in the PR description.
- For UI changes, include screenshots and confirm `yarn --cwd packages/app build` completes.

## eBPF-specific Guidelines (important)
- eBPF code lives under `node-agent/ebpf/` and requires Linux kernel >= 5.4 to compile and test.
- Do not push kernel-loadable changes without validating in an isolated test VM.

Checklist for eBPF PRs:
- The C source compiles locally (use `make` or `./compile-ebpf.sh` in `node-agent/ebpf/`).
- If applicable, attach compiled `.o` objects to the PR or ensure the CI artifacts include them.
- If you ran verifier/load checks, add notes in the PR about the runner used (self-hosted privileged runner) and any `dmesg` errors observed.
- Add a maintainer tag request in the PR for final review if your change touches `node-agent/ebpf/*`.

## CI / Security
- Avoid committing secrets; add a note if your change requires new environment variables or credentials.
- Refer to `.github/copilot-instructions.md` for testing and CI tips specific to this repo.

## Labels & triage
- See `.github/LABELS.md` for a short mapping of labels, examples, and suggested usage.
- When opening issues or PRs, add the most relevant `area-*` label(s) and `eBPF` if applicable; maintainers will adjust as needed.

Thank you — we appreciate your contributions!