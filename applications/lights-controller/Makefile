.PHONY: build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# One shared image for both lights-controller and hub-controller - see
# Dockerfile's top comment.
IMAGE_NAME ?= ghcr.io/liamawhite/lights-controller
TAG ?= dev

build:
	go build ./...

# Build context is the repo root (../..), not this directory - see
# Dockerfile's top comment for why.
docker-build:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) ../..

docker-push:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) --push ../..
