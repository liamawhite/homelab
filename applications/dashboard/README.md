> **Disclaimer:** This repository is mostly vibe-coded, single-user software.

# Dashboard

Application source and multistage container build for `dashboard` in the homelab
monorepo. Run commands below from `applications/dashboard`.

## Development

The multistage Dockerfile installs the build tools and builds both the frontend
and backend. Build the application image with:

```sh
docker build -t dashboard .
```

For local development, use Go (version from `go.mod`) and Node.js 22.

```sh
make build
make test
make format-check
make lint
PORT=8080 go run ./cmd/dashboard
```

`make frontend` builds and prepares embedded assets. Frontend development runs
with `npm run dev --prefix web`.

## Publishing

Changes to `applications/dashboard/` on `main` publish `ghcr.io/liamawhite/dashboard:sha-<full-commit-sha>` for
`linux/amd64` and `linux/arm64`, using `GITHUB_TOKEN` with `packages: write`.
The workflow updates `:latest` only if the built commit is still the head of
`main`. Pull requests run separate format, lint, and test jobs, following
[melodical](https://github.com/liamawhite/melodical/tree/470ea3e5f55c5eb45b722d2d83bfa961bf8450c3/.github/workflows).
Publishing does not deploy to the cluster; homelab selects the image version.
