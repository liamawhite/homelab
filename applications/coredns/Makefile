.PHONY: build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# Default variables
IMAGE_NAME ?= coredns
TAG ?= dev

# Build ARM64 binary for Raspberry Pi
build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o bin/coredns .

# Clean up binaries
clean:
	rm -rf bin

# Build Docker image
docker-build: build
	docker build -t $(IMAGE_NAME):$(TAG) .

# Build and push Docker image
docker-push: build
	docker build -t $(IMAGE_NAME):$(TAG) .
	docker push $(IMAGE_NAME):$(TAG)
