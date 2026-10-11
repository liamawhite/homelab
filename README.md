# homelab

Application source lives under `applications/`: Dashboard, Parental, Shopping,
Lumenetes, and Workouts. Each app is an independent Go module with its own
multistage Dockerfile and frontend dependencies. Run its Makefile from its own
directory; no Nix environment is needed.

Each app has separate check and publishing workflows in `.github/workflows`.
PR format/lint/test checks run only for changes to that app or its check workflow.
Changes to that app or its publishing workflow on `main` publish AMD64/ARM64
images to `ghcr.io/liamawhite/<app>:sha-<commit>`. Each app has a separate build
cache; unrelated infrastructure or other app changes do not trigger its CI.
Workflows can also be started manually.

Publishing uses `GHCR_TOKEN` when configured, otherwise `GITHUB_TOKEN`. Existing
GHCR packages must grant homelab Actions write access to use `GITHUB_TOKEN`.
Application publishing does not deploy to the cluster. Select releases in
`pkg/versions/versions.go` and run the homelab preview/up commands. Dashboard
keeps the existing Home Kubernetes resources and hostname.

The app checks have path filters. Do not require every app's checks on every PR:
GitHub leaves skipped required workflows pending. Require checks only with
appropriate path-scoped rules, or use a separate aggregate gate.

Lumenetes CRD generation uses `applications/lumenetes` by default; set
`LUMENETES_SOURCE` to override it.

## Troubleshooting

### `no IP addresses available in range set`

`ssh` into the node and run `sudo rm -rf /var/lib/cni/networks/cbr0 && sudo reboot`. See [issue](https://github.com/k3s-io/k3s/issues/4682).

### k3s node failes

Check `systemctl status k3s.service` and `journalctl -xeu k3s.service` for details.
