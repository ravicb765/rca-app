#!/usr/bin/env bash
set -euo pipefail

# setup-runner.sh - Helper to provision and register a privileged self-hosted runner
# for this repository with the label 'ebpf'.
#
# Usage:
#   GH_REPO=ravicb765/rca-app GITHUB_TOKEN=ghp_xxx ./scripts/setup-runner.sh
# Or, if 'gh' CLI is configured and authenticated, run without GITHUB_TOKEN.

GH_REPO=${GH_REPO:-ravicb765/rca-app}
WORK_DIR=${WORK_DIR:-/opt/actions-runner}
LABELS=${LABELS:-ebpf}

echo "Setting up self-hosted runner for repo: $GH_REPO"

# Install recommended packages (Ubuntu/Debian)
if command -v apt-get >/dev/null 2>&1; then
  echo "Installing packages via apt"
  sudo apt-get update
  sudo apt-get install -y clang llvm libbpf-dev bpftool make pkg-config libelf-dev git curl jq ca-certificates
else
  echo "Please install: clang, bpftool, libbpf-dev, make, pkg-config, libelf-dev, git, curl, jq"
fi

mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Fetch latest GitHub Actions runner
echo "Downloading latest actions runner..."
LATEST_URL=$(curl -s https://api.github.com/repos/actions/runner/releases/latest | jq -r '.assets[] | select(.name|test("actions-runner-.*-linux-x64-.*.tar.gz")) | .browser_download_url')
if [ -z "$LATEST_URL" ]; then
  echo "Failed to locate runner download URL from GitHub API" >&2
  exit 1
fi
curl -fsSL "$LATEST_URL" -o runner.tar.gz
tar xzf runner.tar.gz
rm runner.tar.gz

# Get registration token
if [ -n "${GITHUB_TOKEN:-}" ]; then
  echo "Getting registration token via REST API"
  TOKEN=$(curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$GH_REPO/actions/runners/registration-token" | jq -r .token)
else
  if command -v gh >/dev/null 2>&1; then
    echo "Getting token via gh CLI"
    TOKEN=$(gh api repos/$GH_REPO/actions/runners/registration-token -X POST --jq .token)
  else
    echo "No GITHUB_TOKEN and no 'gh' CLI available - cannot register runner" >&2
    exit 1
  fi
fi

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Failed to obtain registration token" >&2
  exit 1
fi

# Configure the runner unattended and add label
./config.sh --url "https://github.com/$GH_REPO" --token "$TOKEN" --labels "$LABELS" --unattended --replace --work _work

# Install and start service
sudo ./svc.sh install
sudo ./svc.sh start

# Quick verification
echo "Runner configured. Checking status..."
if command -v gh >/dev/null 2>&1; then
  gh api repos/$GH_REPO/actions/runners --jq '.runners[] | {name: .name, labels: .labels, status: .status}' || true
fi

echo "Runner setup complete. Ensure the runner is online and shows label: $LABELS"

echo "NOTE: This script must be run on an isolated test host (privileged runner). Ensure you understand security implications of running privileged GitHub Actions runners."