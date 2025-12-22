.PHONY: install dev build build-darwin build-windows build-linux clean test lint format help

# Use nix develop shell if nix is available and we're not already in one
ifndef IN_NIX_SHELL
ifneq ($(shell command -v nix 2> /dev/null),)
SHELL := nix develop --command bash
endif
endif

install: ## Install frontend dependencies
	cd frontend && npm install

dev: ## Run application in development mode
	wails dev

build: ## Build application for production
	wails build

build-darwin: ## Build for macOS
	wails build -platform darwin/universal

build-windows: ## Build for Windows
	wails build -platform windows/amd64

build-linux: ## Build for Linux
	wails build -platform linux/amd64

clean: ## Clean build artifacts
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/node_modules

test: ## Run Go tests
	go test ./...

lint: ## Run linters
	cd frontend && npm run lint

format: ## Format code
	go fmt ./...
	cd frontend && npm run format

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@if [ -n "$$(command -v nix 2> /dev/null)" ] && [ -z "$$IN_NIX_SHELL" ]; then \
		echo 'Nix detected - all commands will run in nix develop environment'; \
		echo ''; \
	fi
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
