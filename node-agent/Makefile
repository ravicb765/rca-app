.PHONY: build docker-build clean

BINARY_NAME=node-agent
DOCKER_IMAGE=rca-app-node-agent:latest

build:
	go mod tidy
	go build -o $(BINARY_NAME) main.go

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-amd64 main.go

docker-build:
	docker build -t $(DOCKER_IMAGE) .

clean:
	rm -f $(BINARY_NAME)
	rm -f rca-node-agent.tar.gz

package: build
	mkdir -p dist/ebpf
	cp $(BINARY_NAME) dist/
	cp ebpf/network_tracer.o dist/ebpf/ || true
	cp rca-node-agent.service dist/
	cp scripts/install.sh dist/
	chmod +x dist/scripts/install.sh
	tar -czvf rca-node-agent.tar.gz -C dist .
	rm -rf dist
	@echo "Package created: rca-node-agent.tar.gz"

package-deb: build-amd64
	VERSION=$$(cat ../VERSION 2>/dev/null || echo "0.1.0") ; \
	nfpm pkg --config nfpm.yaml --target rca-node-agent_$$VERSION\_amd64.deb

package-rpm: build-amd64
	VERSION=$$(cat ../VERSION 2>/dev/null || echo "0.1.0") ; \
	nfpm pkg --config nfpm.yaml --target rca-node-agent-$$VERSION-1.x86_64.rpm