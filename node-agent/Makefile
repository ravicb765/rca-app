.PHONY: all build clean tidy docker-build package build-ebpf push

VERSION ?= 1.0
BINARY_NAME=node-agent-$(VERSION)
DOCKER_IMAGE=node-agent:$(VERSION)

all: build

tidy:
	go mod tidy

# Compile eBPF C code to BPF bytecode using clang
# This target generates the missing .o file required by the package step
build-ebpf:
	mkdir -p ebpf
	# We assume the source is at ebpf/network_tracer.c
	clang -O2 -g -target bpf -D__TARGET_ARCH_$(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/') -I/usr/include/$(shell uname -m)-linux-gnu -c ebpf/network_tracer.c -o ebpf/network_tracer.o

# Build the Go binary, ensuring eBPF object is built first
build: tidy build-ebpf
	CGO_ENABLED=0 GOOS=linux go build -o node-agent main.go

# Package the binary and the eBPF object file
package: build
	mkdir -p dist/ebpf
	cp $(BINARY_NAME) dist/
	cp ebpf/network_tracer.o dist/ebpf/

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f ebpf/*.o
	rm -rf dist
	docker rmi $(DOCKER_IMAGE) || true

docker-build: tidy build-ebpf build package
	docker build --no-cache -t $(DOCKER_IMAGE) .

push: docker-build
	docker push $(DOCKER_IMAGE)
