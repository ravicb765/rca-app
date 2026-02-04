#!/usr/bin/env bash
set -euo pipefail

# Simple runner validation script to be run on a privileged self-hosted runner.
# It compiles an example BPF program, attempts to load it, and collects logs under ./verify-logs.

mkdir -p verify-logs
EXAMPLE_C=node-agent/ebpf/examples/fixed_stack.c
OUT_O=node-agent/ebpf/runner-test.o

echo "Compiling $EXAMPLE_C -> $OUT_O" | tee verify-logs/compile.log
clang -O2 -target bpf -c "$EXAMPLE_C" -o "$OUT_O" 2>&1 | tee -a verify-logs/compile.log || true

# Try to load
echo "Attempt load" | tee verify-logs/load.log
sudo bpftool prog load "$OUT_O" /sys/fs/bpf/runner_test > verify-logs/load.out 2>&1 || true
sudo bpftool prog show >> verify-logs/load.out 2>&1 || true

# Collect kernel and bpftool state
sudo dmesg -T | tail -n 200 > verify-logs/dmesg-after.log || true
sudo bpftool prog show > verify-logs/bpftool-prog-show.log || true
sudo bpftool map show > verify-logs/bpftool-map-show.log || true
sudo ls -la /sys/fs/bpf > verify-logs/bpf_fs_listing.log || true

# Cleanup if loaded
sudo rm -f /sys/fs/bpf/runner_test || true

echo "Runner validation logs written to verify-logs/"
