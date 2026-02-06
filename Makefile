.PHONY: all server node-agent cluster-agent backstage-portal push

VERSION ?= 1.0

all: server node-agent cluster-agent backstage-portal

server:
	@echo "Building Server..."
	$(MAKE) -C server VERSION=$(VERSION)

node-agent:
	@echo "Building Node Agent..."
	$(MAKE) -C node-agent VERSION=$(VERSION)

cluster-agent:
	@echo "Building Cluster Agent..."
	$(MAKE) -C cluster-agent VERSION=$(VERSION)

backstage-portal:
	@echo "Building Backstage Portal..."
	$(MAKE) -C backstage-portal VERSION=$(VERSION)

clean:
	@echo "Cleaning..."
	$(MAKE) -C server clean
	$(MAKE) -C node-agent clean
	$(MAKE) -C cluster-agent clean
	$(MAKE) -C backstage-portal clean
	rm -rf dist

push:
	@echo "Pushing images..."
	$(MAKE) -C server push VERSION=$(VERSION)
	$(MAKE) -C node-agent push VERSION=$(VERSION)
	$(MAKE) -C cluster-agent push VERSION=$(VERSION)
	$(MAKE) -C backstage-portal push VERSION=$(VERSION)