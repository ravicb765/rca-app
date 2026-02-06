# Maintainers

This document lists current maintainers for RCA-App and how to request reviews or become a maintainer.

## Primary Maintainers
- **@ravicb765** — Overall project, eBPF, node-agent, server, ML service, Backstage plugins

> If a primary maintainer is not available, open a PR as usual and request review from another team member or use GitHub Discussions to find a reviewer.

## Areas and Contacts
- area/node-agent (eBPF, DaemonSet, collectors): @ravicb765
- area/server (API, service map builder, inspections): @ravicb765
- area/ml (RCA models, `/analyze` API): @ravicb765
- area/ui (Backstage plugins, frontend): @ravicb765
- docs & CI: @ravicb765

## Urgent issues
- For urgent security or production eBPF problems, create a GitHub Issue with label `urgent` and tag `@ravicb765` and any other on-call contacts.
- For urgent PR reviews, add the `urgent-review` label and mention maintainers in the PR.

## Becoming a Maintainer
1. Demonstrate consistent contributions to one or more areas (3+ merged PRs in those directories).
2. Discuss on GitHub Discussions or directly with existing maintainers and request to be added.
3. A maintainer will add you to `MAINTAINERS.md` and `.github/CODEOWNERS` (if appropriate).

## Process Notes
- Keep changes small and scoped; large changes should be split into follow-ups.
- eBPF changes must include compiled `.o` artifacts or CI artifacts and follow the eBPF checklist in `CONTRIBUTING.md`.

If you'd like, I can also create the suggested GitHub labels (`area/node-agent`, `eBPF`, `urgent-review`).