# Simple Makefile for compiling eBPF sources

SRCS := $(wildcard *.c)
OBJS := $(SRCS:.c=.o)

all: $(OBJS)

%.o: %.c
	clang -O2 -target bpf -c $< -o $@

syntax-check:
	clang -fsyntax-only $(SRCS)

clean:
	rm -f $(OBJS)

.PHONY: all syntax-check clean
