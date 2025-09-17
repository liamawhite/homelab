.PHONY: build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# Default variables
IMAGE_NAME ?= homeassistant
TAG ?= dev
HOME_ASSISTANT_VERSION ?= 2025.9.3
ADAPTIVE_LIGHTING_VERSION ?= v1.26.0

# Build Docker image for ARM64 (builds inside container)
docker-build:
	docker buildx build --platform linux/arm64 -t $(IMAGE_NAME):$(TAG) \
		--build-arg HOME_ASSISTANT_VERSION=$(HOME_ASSISTANT_VERSION) \
		--build-arg ADAPTIVE_LIGHTING_VERSION=$(ADAPTIVE_LIGHTING_VERSION) .

# Build and push Docker image for ARM64
docker-push:
	docker buildx build --platform linux/arm64 -t $(IMAGE_NAME):$(TAG) \
		--build-arg HOME_ASSISTANT_VERSION=$(HOME_ASSISTANT_VERSION) \
		--build-arg ADAPTIVE_LIGHTING_VERSION=$(ADAPTIVE_LIGHTING_VERSION) --push .