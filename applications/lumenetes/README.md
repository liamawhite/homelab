> **Disclaimer:** This repository is mostly vibe-coded, single-user software.

# Lumenetes

Application source and multistage container build for `lumenetes` in the homelab
monorepo. Run commands below from `applications/lumenetes`.

## Development

The multistage Dockerfile installs the build tools and builds both the frontend
and backend. Build the application image with:

```sh
docker build -t lumenetes .
```

For local development, use Go (version from `go.mod`) and Node.js 22.

```sh
make build
make test
make format-check
make lint
```

`make frontend` builds the React UI and copies it into the Go embed directory.
Generated Go and TypeScript bindings are committed; use `make generate` after
changing protobufs, or Kubernetes API types.

## Container publishing

Changes to `applications/lumenetes/` on `main` publish `ghcr.io/liamawhite/lumenetes:sha-<full-commit-sha>`
for `linux/amd64` and `linux/arm64`. The workflow updates `:latest` only while the
built commit remains the head of `main`. Publishing uses the `GHCR_TOKEN` Actions secret for the existing package, falling
back to the repository's `GITHUB_TOKEN` when no registry secret is configured.
Pull requests run independent format, lint, and backend test jobs.
The workflows follow [melodical](https://github.com/liamawhite/melodical/tree/470ea3e5f55c5eb45b722d2d83bfa961bf8450c3/.github/workflows).

Deploy an immutable SHA tag or digest from homelab; publishing an image does not
apply changes to the cluster.

The shared image contains `/lumenetes-controller` and `/hub-controller`; choose
the binary with the container command. `cmd/lumenetes-lights` is the separate
Hue management CLI, compatible with homelab’s `infra.yaml` configuration.

The existing package predates this repository and initially rejected its
`GITHUB_TOKEN` with `permission_denied: write_package`. The `GHCR_TOKEN` Actions
secret supplies the existing homelab publishing credential without storing it in
source. To switch to `GITHUB_TOKEN`, grant the homelab repository **Write** permission
under [Manage Actions access](https://github.com/users/liamawhite/packages/container/lumenetes/settings)
and remove the `GHCR_TOKEN` secret.
