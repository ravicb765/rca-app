#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE"

if ! command -v clang >/dev/null 2>&1; then
  echo "clang not found. Install clang/llvm to compile eBPF examples." >&2
  exit 2
fi

for f in *.c; do
  o="${f%.c}.o"
  echo "Compiling $f -> $o"
  clang -O2 -target bpf -c "$f" -o "$o" 2>"${f%.c}.compile.log" || true
  if [ -s "${f%.c}.compile.log" ]; then
    echo "--- compiler output for $f ---"
    tail -n 100 "${f%.c}.compile.log"
    echo "-----------------------------"
  fi
done

echo "Done. Generated .o files (if compilation succeeded)."
ls -1 *.o || true
