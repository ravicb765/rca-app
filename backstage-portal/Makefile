.PHONY: build docker-build clean

build:
	yarn install
	yarn tsc
	yarn build:backend

docker-build:
	docker build -t rca-app/backstage-portal:latest .

clean:
	rm -rf dist packages/*/dist
