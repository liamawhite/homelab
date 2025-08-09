.PHONY: build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# Default variables
IMAGE_NAME ?= coredns
TAG ?= dev

# Build Docker image (builds inside container)
docker-build:
	docker build -t $(IMAGE_NAME):$(TAG) .

# Build and push Docker image
docker-push:
	docker build -t $(IMAGE_NAME):$(TAG) .
	docker push $(IMAGE_NAME):$(TAG)
