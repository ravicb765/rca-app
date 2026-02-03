#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE"

if ! command -v clang >/dev/null 2>&1; then
  echo "clang not found. Install clang/llvm to compile eBPF examples." >&2
  exit 2
fi

mkdir -p logs

for f in *.c; do
  o="${f%.c}.o"
  log="logs/${f%.c}.compile.log"
  echo "Compiling $f -> $o (log: $log)"
  clang -O2 -target bpf -c "$f" -o "$o" 2>"$log" || true
  if [ -s "$log" ]; then
    echo "--- compiler output for $f (tail) ---"
    tail -n 80 "$log"
    echo "-------------------------------------"
  else
    echo "Compiled $f cleanly"
  fi
done

echo "Compilation logs are in: $(pwd)/logs"
ls -la logs || true
