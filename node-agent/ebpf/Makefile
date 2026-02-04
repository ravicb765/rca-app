CLANG ?= clang
BPFTOOL ?= bpftool

ARCH := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/' | sed 's/ppc64le/powerpc/' | sed 's/mips.*/mips/')

TARGETS := network_tracer.o

.PHONY: all clean

all: $(TARGETS)

clean:
	rm -f $(TARGETS) vmlinux.h

vmlinux.h:
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

network_tracer.o: network_tracer.c vmlinux.h
	$(CLANG) -O2 -g -target bpf -D__TARGET_ARCH_$(ARCH) -I. -c $< -o $@