export GOWORK := off

GO_PACKAGES := ./cmd/... ./internal/... ./web

.PHONY: build frontend generate test test-browser run dev static-linux format format-check lint

dev: frontend
	CGO_ENABLED=0 go run ./cmd/dev

frontend:
	cd web && npm ci && npm run build

generate:
	sqlc generate

build: frontend
	CGO_ENABLED=0 go build -trimpath -o bin/melodical ./cmd/melodical

run: build
	./bin/melodical

test: frontend
	CGO_ENABLED=0 go test $(GO_PACKAGES)
	CGO_ENABLED=0 go vet $(GO_PACKAGES)

test-browser: build
	cd web && npx playwright test

static-linux: frontend
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o bin/melodical-linux-amd64 ./cmd/melodical

format:
	gofmt -w cmd internal web/embed.go
	cd web && npm run format

format-check:
	@unformatted=$$(gofmt -l cmd internal web/embed.go) || exit 1; if [ -n "$$unformatted" ]; then printf '%s\n' "$$unformatted"; exit 1; fi
	cd web && npm run format:check

lint: frontend
	CGO_ENABLED=0 golangci-lint run $(GO_PACKAGES)
	cd web && npm run lint
