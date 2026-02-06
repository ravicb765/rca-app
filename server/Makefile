.PHONY: all build clean tidy docker-build push

VERSION ?= 1.0
BINARY_NAME=server
DOCKER_IMAGE=rca-app-server:$(VERSION)

all: build

tidy:
	go mod tidy

build: tidy
	CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -tags musl -ldflags="-s -w" -o $(BINARY_NAME) main.go

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf dist
	docker rmi $(DOCKER_IMAGE) || true

docker-build: tidy
	docker build --no-cache -t $(DOCKER_IMAGE) .

push: docker-build
	docker push $(DOCKER_IMAGE)