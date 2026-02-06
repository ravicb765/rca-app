.PHONY: all build clean tidy docker-build push

VERSION ?= 1.0
BINARY_NAME=cluster-agent
DOCKER_IMAGE=clusteragent:$(VERSION)

all: build

tidy:
	go mod tidy

build: clean tidy
	CGO_ENABLED=0 GOOS=linux go build -o $(BINARY_NAME) ./

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf dist
	docker rmi $(DOCKER_IMAGE) || true

docker-build: tidy
	docker build --no-cache -t $(DOCKER_IMAGE) .

push: docker-build
	docker push $(DOCKER_IMAGE)