#!/bin/bash
set -e

echo "Generating vmlinux.h..."

# Dump kernel BTF to vmlinux.h
# This requires a kernel with CONFIG_DEBUG_INFO_BTF=y (standard on most modern distros)
bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

echo "Done. vmlinux.h created."