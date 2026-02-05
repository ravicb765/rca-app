.PHONY: all build docker-build clean tidy

all: build

build:
	$(MAKE) -C server build
	$(MAKE) -C node-agent build
	$(MAKE) -C ml-service build

docker-build:
	$(MAKE) -C server docker-build
	$(MAKE) -C node-agent docker-build
	$(MAKE) -C ml-service docker-build

clean:
	$(MAKE) -C server clean
	$(MAKE) -C node-agent clean
	$(MAKE) -C ml-service clean

tidy:
	cd server && go mod tidy
	cd node-agent && go mod tidy