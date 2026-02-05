.PHONY: build ebpf docker-build clean

BINARY_NAME=node-agent
DOCKER_IMAGE=rca-app-node-agent:latest

build: ebpf
	go mod tidy
	go build -tags ebpf -o $(BINARY_NAME) .

ebpf:
	$(MAKE) -C ebpf

docker-build:
	docker build -t $(DOCKER_IMAGE) .

clean:
	rm -f $(BINARY_NAME)
	$(MAKE) -C ebpf clean