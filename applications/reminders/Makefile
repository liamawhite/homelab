SHELL := nix develop --command bash -c
IMAGE_NAME ?= ghcr.io/liamawhite/reminders
TAG ?= dev

gen:
	buf generate
	sqlc generate

build: gen
	cd web && npm ci && npm run build
	cp -r web/dist/. internal/webui/dist/
	go build ./...

docker-build:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) .

docker-push:
	docker buildx build --platform linux/arm64 -f Dockerfile -t $(IMAGE_NAME):$(TAG) --push .
