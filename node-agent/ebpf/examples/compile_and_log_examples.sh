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

  EXTRA_INCLUDES=""
  # Add common system include locations so clang can find asm/types.h and kernel headers
  if [ -d "/usr/include/x86_64-linux-gnu" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/include/x86_64-linux-gnu"
  fi
  if [ -d "/usr/src/linux-headers-$(uname -r)/include" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/include"
  fi
  # Add architecture-specific and generated include paths which are often required
  # for asm/* headers like rwonce.h and types.h on Ubuntu kernels
  if [ -d "/usr/src/linux-headers-$(uname -r)/arch/x86/include" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/arch/x86/include"
  fi
  if [ -d "/usr/src/linux-headers-$(uname -r)/arch/x86/include/generated" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/arch/x86/include/generated"
  fi
  if [ -d "/usr/src/linux-headers-$(uname -r)/include/generated" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/include/generated"
  fi

  # Define kernel/BPF macros so kernel headers expose types like u64/u32
  # Add small portability defines used by the examples (u64/u32) and include
  # POSIX types for ssize_t. These make the examples compile on runners where
  # kernel headers don't provide 'u64' aliases.
  # Avoid pulling in glibc multilib headers; define lightweight aliases instead
  CFLAGS="-D__KERNEL__ -D__BPF_TRACING__ -include linux/types.h -D u64=__u64 -D u32=__u32 -D ssize_t=long"
  clang -O2 -target bpf $CFLAGS -I/usr/include $EXTRA_INCLUDES -c "$f" -o "$o" 2>"$log" || true
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
