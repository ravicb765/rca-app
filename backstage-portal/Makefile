.PHONY: all docker-build push clean

VERSION ?= 1.0
DOCKER_IMAGE=backstage-portal:$(VERSION)

all: docker-build

docker-build:
	docker build --no-cache -t $(DOCKER_IMAGE) .

push: docker-build
	docker push $(DOCKER_IMAGE)

clean:
	docker rmi $(DOCKER_IMAGE) || true