.PHONY: build docker-build clean

DOCKER_IMAGE=rca-app-ml-service:latest

build:
	pip install -r requirements.txt

docker-build:
	docker build -t $(DOCKER_IMAGE) .

clean:
	rm -rf __pycache__