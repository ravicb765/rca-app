#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE"

if ! command -v clang >/dev/null 2>&1; then
  echo "clang not found. Install clang/llvm to compile eBPF sources." >&2
  exit 2
fi

for f in *.c; do
  o="${f%.c}.o"
  echo "Compiling $f -> $o"
  clang -O2 -target bpf -c "$f" -o "$o"
done

echo "Compilation complete. Generated:"
ls -1 *.o || true
