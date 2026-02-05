.PHONY: build docker-build clean test

BINARY_NAME=server
DOCKER_IMAGE=rca-app-server:latest

build:
	go mod tidy
	CGO_ENABLED=1 go build -o $(BINARY_NAME) main.go

test:
	go test ./...

docker-build:
	docker build -t $(DOCKER_IMAGE) .

clean:
	rm -f $(BINARY_NAME)