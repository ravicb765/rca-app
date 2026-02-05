CLANG ?= clang
CFLAGS := -O2 -g -target bpf -D__TARGET_ARCH_x86

# List all C files to compile
TARGETS := network_tracer.o http_tracer.o

all: $(TARGETS)

%.o: %.c
	$(CLANG) $(CFLAGS) -c $< -o $@

clean:
	rm -f *.o