.PHONY: build docker-build clean

BINARY_NAME=server
DOCKER_IMAGE=rca-app-server:latest

build:
	go mod tidy
	go build -o $(BINARY_NAME) main.go

docker-build:
	docker build -t $(DOCKER_IMAGE) .

clean:
	rm -f $(BINARY_NAME)