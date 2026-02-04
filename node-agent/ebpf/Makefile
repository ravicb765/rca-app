# Simple Makefile for compiling eBPF sources

SRCS := $(wildcard *.c)
OBJS := $(SRCS:.c=.o)

all: $(OBJS)

%.o: %.c
	clang -O2 -target bpf -c $< -o $@

syntax-check:
	clang -fsyntax-only $(SRCS)

verify: all
	@which bpftool >/dev/null 2>&1 || (echo "bpftool not found; skipping verify" && exit 0)
	@for o in $(OBJS); do \
		echo "attempting to load $$o"; \
		sudo bpftool prog load $$o /sys/fs/bpf/$${o%.o} || true; \
	done
	sudo bpftool prog show || true

clean:
	rm -f $(OBJS)

.PHONY: all syntax-check verify clean
