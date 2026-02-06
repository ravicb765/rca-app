#!/usr/bin/env bash
set -euo pipefail

# run-local-agent.sh
# Compile BPF examples, build the node-agent (optional ebpf support), and run locally.
# Usage:
#   ./scripts/run-local-agent.sh [--ebpf] [--examples] [--build] [--run] [--clean]
# Options (default if none provided: --examples --build --run)

PROG=$(basename "$0")
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
EBPF=false
COMPILE_EXAMPLES=false
BUILD_AGENT=false
RUN_AGENT=false
CLEAN=false

if [ $# -eq 0 ]; then
  COMPILE_EXAMPLES=true
  BUILD_AGENT=true
  RUN_AGENT=true
fi

while [ $# -gt 0 ]; do
  case "$1" in
    --ebpf) EBPF=true; shift ;;
    --examples) COMPILE_EXAMPLES=true; shift ;;
    --build) BUILD_AGENT=true; shift ;;
    --run) RUN_AGENT=true; shift ;;
    --clean) CLEAN=true; shift ;;
    -h|--help) echo "Usage: $PROG [--ebpf] [--examples] [--build] [--run] [--clean]"; exit 0 ;;
    *) echo "Unknown arg: $1"; echo "Use -h for help"; exit 1 ;;
  esac
done

cd "$ROOT_DIR"

check_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "required: $1 not found in PATH" >&2; exit 2; }
}

if $COMPILE_EXAMPLES; then
  echo "[*] Compiling eBPF examples"
  check_cmd clang
  mkdir -p node-agent/ebpf
  for c in node-agent/ebpf/examples/*.c; do
    o=${c%.c}.o
    echo " - $c -> $o"
    clang -O2 -target bpf -c "$c" -o "$o" || { echo "clang failed for $c" >&2; exit 3; }
  done
  echo "[*] Compiled examples"
fi

if $CLEAN; then
  echo "[*] Cleaning compiled objects and binaries"
  rm -f node-agent/ebpf/*.o
  rm -f node-agent/node-agent node-agent/node-agent-ebpf || true
  exit 0
fi

if $BUILD_AGENT; then
  echo "[*] Building node-agent"
  if $EBPF; then
    echo " - Building with ebpf tag (requires cilium/ebpf in module cache)"
    (cd node-agent && go build -tags ebpf -o node-agent-ebpf) || { echo "go build (ebpf) failed" >&2; exit 4; }
    AGENT_BIN="$(pwd)/node-agent/node-agent-ebpf"
  else
    (cd node-agent && go build -o node-agent) || { echo "go build failed" >&2; exit 4; }
    AGENT_BIN="$(pwd)/node-agent/node-agent"
  fi
  echo "[*] Built agent: $AGENT_BIN"
fi

if $RUN_AGENT; then
  echo "[*] Running agent"
  if [ -z "${AGENT_BIN:-}" ]; then
    echo "Agent binary not found. Did you run with --build?" >&2
    exit 5
  fi

  # Kernel check for ebpf mode
  if $EBPF; then
    if [ "$(uname -s)" != "Linux" ]; then
      echo "EBPF mode requires Linux. Exiting." >&2
      exit 6
    fi
    KVER=$(uname -r | awk -F. '{print $1"."$2}')
    echo "Kernel: $KVER"
    # Simple numeric check, require >= 5.4
    req="5.4"
    if awk 'BEGIN{exit ARGV[1]<ARGV[2]}' "$KVER" "$req"; then
      echo "Warning: kernel $KVER may be older than $req; eBPF features may be limited." >&2
    fi
    # bpftool check
    check_cmd bpftool
    echo "Starting agent with sudo (ebpf mode)"
    sudo "$AGENT_BIN" &
    AGENT_PID=$!
  else
    "$AGENT_BIN" &
    AGENT_PID=$!
  fi

  echo "Agent started (pid=$AGENT_PID). Logs will be attached to this terminal. Press Ctrl-C to stop."
  trap 'echo "Stopping agent (pid=$AGENT_PID)"; kill $AGENT_PID 2>/dev/null || true; wait $AGENT_PID 2>/dev/null || true; exit 0' INT TERM
  wait $AGENT_PID
fi

echo "Done"
