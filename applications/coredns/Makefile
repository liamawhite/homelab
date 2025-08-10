.PHONY: build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# Default variables
IMAGE_NAME ?= coredns
TAG ?= dev

# Build Docker image for ARM64 (builds inside container)
docker-build:
	docker buildx build --platform linux/arm64 -t $(IMAGE_NAME):$(TAG) .

# Build and push Docker image for ARM64
docker-push:
	docker buildx build --platform linux/arm64 -t $(IMAGE_NAME):$(TAG) --push .
