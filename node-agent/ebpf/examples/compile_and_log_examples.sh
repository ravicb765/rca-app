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
  # Prefer user-space libbpf headers if available (avoids pulling in full kernel
  # arch headers that contain inline asm constraints which clang may reject).
  if [ -d "/usr/include/bpf" ]; then
    EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/include -I/usr/include/bpf"
  else
    # Fall back to distro-provided user-space include dirs
    if [ -d "/usr/include/x86_64-linux-gnu" ]; then
      EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/include/x86_64-linux-gnu"
    fi
    # As a last resort, add kernel header locations (may pull in arch asm)
    if [ -d "/usr/src/linux-headers-$(uname -r)/include" ]; then
      EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/include"
    fi
    # Add architecture-specific and generated include paths (only if kernel headers are used)
    if [ -d "/usr/src/linux-headers-$(uname -r)/arch/x86/include" ]; then
      EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/arch/x86/include"
    fi
    if [ -d "/usr/src/linux-headers-$(uname -r)/arch/x86/include/generated" ]; then
      EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/arch/x86/include/generated"
    fi
    if [ -d "/usr/src/linux-headers-$(uname -r)/include/generated" ]; then
      EXTRA_INCLUDES="$EXTRA_INCLUDES -I/usr/src/linux-headers-$(uname -r)/include/generated"
    fi
  fi

  # Define kernel/BPF macros so kernel headers expose types like u64/u32
  # Add small portability defines used by the examples (u64/u32) and include
  # POSIX types for ssize_t. These make the examples compile on runners where
  # kernel headers don't provide 'u64' aliases.
  # Set compile flags based on available headers. If libbpf user-space
  # headers are available, prefer small type aliases (avoid -include linux/types.h
  # which pulls in asm headers that can contain arch inline asm). Otherwise
  # fall back to including linux/types.h and kernel aliases.
  if [ -d "/usr/include/bpf" ]; then
    # Use a small compatibility header to provide __u64/__u32/u64/u32/ssize_t
    # to avoid forcing inclusion of full kernel arch headers (with inline asm)
    CFLAGS="-D__BPF_TRACING__ -include ${HERE}/compat_types.h"
    # Add the local include dir first so our small asm/types.h stub satisfies
    # <asm/types.h> includes without pulling in full kernel arch headers.
    INCS="-I${HERE}/include -I/usr/include -I/usr/include/bpf $EXTRA_INCLUDES"
  else
    CFLAGS="-D__KERNEL__ -D__BPF_TRACING__ -include linux/types.h -D u64=__u64 -D u32=__u32 -D ssize_t=long"
    INCS="-I/usr/include $EXTRA_INCLUDES"
  fi

  clang -O2 -target bpf $CFLAGS $INCS -c "$f" -o "$o" 2>"$log" || true
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
