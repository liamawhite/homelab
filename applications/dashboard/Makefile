GO_PACKAGES := ./cmd/... ./internal/...
IMAGE_NAME ?= ghcr.io/liamawhite/dashboard
TAG ?= dev
export CGO_ENABLED := 0
export GOWORK := off

.PHONY: frontend build test format format-check lint docker-build docker-push

frontend:
	cd web && npm ci && npm run build
	mkdir -p internal/webui/dist
	cp -r web/dist/. internal/webui/dist/

build: frontend
	go build $(GO_PACKAGES)

test: frontend
	go test $(GO_PACKAGES)
	go vet $(GO_PACKAGES)

format:
	gofmt -w cmd internal
	cd web && npm run format

format-check:
	@unformatted=$$(gofmt -l cmd internal) || exit 1; if [ -n "$$unformatted" ]; then printf '%s\n' "$$unformatted"; exit 1; fi
	cd web && npm run format:check

lint: frontend
	golangci-lint run $(GO_PACKAGES)
	cd web && npm run lint

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME):$(TAG) .

docker-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME):$(TAG) --push .
