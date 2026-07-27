.PHONY: gen build clean docker-build docker-push

# Use nix develop shell for all commands
SHELL := nix develop --command bash -c

# One shared image for both lumenetes-controller and hub-controller (plus
# the read-only web UI lumenetes-controller itself now serves) - see
# Dockerfile's top comment.
IMAGE_NAME ?= ghcr.io/liamawhite/lumenetes
TAG ?= dev

gen:
	buf generate

# Builds web/ and embeds it into internal/webui/dist before compiling -
# only lumenetes-controller actually go:embeds that directory (see
# internal/webui/embed.go), but running this unconditionally is simpler than
# threading a binary-specific build step through.
build: gen
	cd web && npm ci && npm run build
	cp -r web/dist/. internal/webui/dist/
	go build ./...

# Build context is the repo root (../..), not this directory - see
# Dockerfile's top comment for why.
docker-build:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) ../..

docker-push:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) --push ../..
