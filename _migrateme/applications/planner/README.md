# Planner

A Wails desktop application built with Go and React.

## Prerequisites

- Nix with flakes enabled
- Git

## Development

The project uses Nix flakes to manage the development environment, including Wails CLI, Go, and Node.js.

### Getting Started

1. Install frontend dependencies:
   ```bash
   make install
   ```

2. Run the development server:
   ```bash
   make dev
   ```

The Makefile automatically uses the Nix environment, so you can run `make dev` directly without entering `nix develop` manually.

### Live Development

When you run `make dev`, Wails will start:
- A Vite development server with hot reload for frontend changes
- A dev server at http://localhost:34115 where you can call your Go methods from the browser devtools

### Building

Build for production:
```bash
make build
```

Build for specific platforms:
```bash
make build-darwin   # macOS universal binary
make build-windows  # Windows amd64
make build-linux    # Linux amd64
```

### Other Commands

- `make clean` - Clean build artifacts
- `make test` - Run Go tests
- `make lint` - Run linters
- `make format` - Format code
- `make help` - Show all available commands

## Architecture

- **Backend**: Go with Wails
- **Frontend**: React + TypeScript + Vite
- **Build Tool**: Wails CLI
- **Package Manager**: npm (frontend), Go modules (backend)

## Nix Environment

The `flake.nix` provides:
- Wails CLI
- Go toolchain
- Node.js 20
- Build tools (pkg-config)
- macOS frameworks (when on Darwin)

The environment is automatically activated when running make commands outside of a Nix shell.

## Application Icon

The application uses a custom calendar-themed icon:

- **Source**: `logo.svg` - Vector logo (editable)
- **macOS**: `build/darwin/iconfile.icns` - macOS app icon
- **General**: `build/appicon.png` - PNG icon (512x512)

The icon features:
- Indigo blue background with white calendar grid
- Highlighted date indicator (15th)
- Green checkmark for completed tasks
- Modern, clean design optimized for macOS, Windows, and Linux

## Configuration

Project settings can be configured in `wails.json`. More information: https://wails.io/docs/reference/project-config
