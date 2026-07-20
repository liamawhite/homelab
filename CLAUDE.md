# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

The project uses Nix for development environment management, Go for CLI/infrastructure, and Yarn for TypeScript package management.

**Go CLI Commands:**
- `go run cli/main.go bootstrap --node <name>` - Bootstrap a Raspberry Pi node
- `go run cli/main.go k3s --node <name> --cluster-init` - Initialize K3s cluster
- `go run cli/main.go k3s --node <name> --server <url>` - Join node to cluster (cluster token is fetched automatically from the join server and saved to infra.yaml if not already set)
- `go run cli/main.go kubeconfig [--node <name>]` - Extract kubeconfig (connects via the cluster VIP if --node omitted)
- `go run cli/main.go node status` - Show health/status for each node
- `go run cli/main.go preview` - Preview Pulumi infrastructure changes (kube-vip, Istio control plane + shared ingress Gateway)
- `go run cli/main.go up` - Deploy Pulumi infrastructure (kube-vip, Istio control plane + shared ingress Gateway)

There is no `pulumi/` project directory and no raw `pulumi` CLI workflow - `up`/`preview` define the project, backend, and program entirely in Go and run fully inline via the Automation API (see below). There's no `destroy` command yet; since there's no on-disk Pulumi.yaml anymore, doing it via the raw `pulumi` CLI would need a throwaway project file pointed at `.pulumi-state/` - add a `homelab destroy` command (mirroring `cli/cmd/pulumi/up.go`) if/when that's needed.

**TypeScript Development Commands:**
- `nix develop` - Enter the development shell with all required tools
- `yarn check` - Run code generation and formatting (equivalent to `yarn gen && yarn format`)
- `yarn gen` - Generate Kubernetes CRD TypeScript bindings from upstream sources
- `yarn format` - Format all code using Prettier
- `yarn deploy` - Deploy the homelab infrastructure using Pulumi (runs `yarn homeassistant` first)
- `yarn homeassistant` - Extract Home Assistant configuration from running pod
- `yarn passwords` - Extract and display secrets from the Pulumi stack
- `yarn k9s` - Launch k9s Kubernetes dashboard using the generated kubeconfig

All commands should be run within the Nix development shell (`nix develop`) which provides:

- crd2pulumi (for generating TypeScript from Kubernetes CRDs)
- kubernetes-helm, k9s, jq, yq, git, nodejs_24, go, docker, gnumake

## Project Architecture

This is a hybrid Infrastructure as Code project that deploys a complete homelab on Raspberry Pi hardware using:
- **Go CLI** (`cli/`) - Bootstraps infrastructure (Raspberry Pi provisioning, K3s installation) and deploys Pulumi-managed components (kube-vip for control plane HA), fully inline via the Automation API
- **Pulumi TypeScript** (root/`project/`) - Legacy deployment for applications (being gradually migrated to Go)

### Go CLI and Infrastructure (`cli/`, `pkg/`)

`cli/` only holds the Cobra command wiring. `pkg/` splits three ways:
- **`pkg/*`** (root) - cross-cutting utilities: `config`, `kubeconfig`, `versions`, `k3s`, `ssh`, `probe`, `raspberry`, and the orchestrator `deploy`.
- **`pkg/crds/*`** - generated CRD types + the raw manifest + `InstallCRDs` helpers, one subdirectory per upstream CRD source (`istio`, `gatewayapi`). These don't register as Pulumi `ComponentResource`s - `InstallCRDs` just applies an embedded manifest and returns a `*yamlv2.ConfigGroup` directly - so they aren't "components" in the same sense as the next bucket.
- **`pkg/components/*`** - actual `ComponentResource`-registering code that deploys real workloads (`kubevip`, `longhorn`, `cloudflare/{tunnel,auth}`, `istio` + `istio/gateway` + `istio/route`).

**Namespace convention**: components never create their own namespace. Every namespace is created once, centrally, by `pkg/deploy/namespaces.go`; components that need one take a `Namespace pulumi.StringInput` arg instead. This exists because `pkg/crds/istio.InstallCRDs` and `pkg/components/istio.NewIstio` used to each create their own `istio-system` `Namespace` object - two Pulumi resources owning one physical namespace, which conflicts the moment both are wired in. Follow this pattern for any new component that needs a namespace: add/uncomment its entry in `namespaces.go`, thread the name through as an arg, and pass `pulumi.DependsOn` on the actual namespace resource at the call site (Helm chart resources don't always reliably propagate implicit Output-based dependencies).

1. **CLI Tool** (`cli/`):
   - `cmd/bootstrap.go` - Raspberry Pi provisioning (SSH keys, system updates)
   - `cmd/k3s.go` - K3s installation (cluster init or join; auto-fetches and saves the cluster token to infra.yaml if unset)
   - `cmd/kubeconfig.go` - Kubeconfig extraction (merges into your default kubeconfig by default; connects via the VIP if --node omitted)
   - `cmd/node.go`, `cmd/node_status.go` - Per-node health status (ping/SSH/bootstrap/k3s/API checks)
   - `cmd/pulumi/` - The `up`/`preview` commands (still top-level: `homelab up`, `homelab preview`), in their own subpackage:
     - `pulumi.go` - Shared setup: resolves a reachable cluster endpoint, extracts a kubeconfig from it, and builds a fully inline Automation API stack from `pkg/deploy.Program` - project name, Go runtime, and the `.pulumi-state/` backend URL are all defined in Go here, no Pulumi.yaml involved
     - `up.go`, `preview.go` - Run `pulumi up`/`pulumi preview` in-process via the Automation API

2. **Root `pkg/` utilities**:
   - `config/config.go` - Shared `infra.yaml` loader (VIP, nodes, SSH, cluster token, Cloudflare account/token/tunnel domain/Access allowed emails); precedence: CLI flags > infra.yaml > env vars > defaults
   - `kubeconfig/kubeconfig.go` - Kubeconfig path resolution (`$KUBECONFIG`, else `~/.kube/config`) and the `homelab` context name, used by the CLI's `kubeconfig` command
   - `versions/versions.go` - Pinned component versions (kube-vip, Istio, GatewayAPI, Longhorn) - plain Go constants, no Pulumi config involved. `pkg/crds/*/gen-crds.sh` scripts read their target version from here at generation time (`make sync`), so bumping a version and regenerating always stay in sync.
   - `k3s/` - K3s installer, kubeconfig extraction, and `resolve.go`'s `ResolveClusterEndpoint` - tries the cluster VIP first, then each node directly, returning the first that accepts an SSH connection (errors if none do). This matters because the VIP only exists once kube-vip is deployed, so the very first `up` has to reach a node directly.
   - `ssh/` - SSH client for remote operations
   - `raspberry/` - Raspberry Pi bootstrap/provisioning checks
   - `probe/` - Network reachability/neighbor-table probing used by `node status`
   - `deploy/` - The Pulumi program itself, as a library:
     - `deploy.go` - `Program(kubeconfig, infraCfg)` returns a `pulumi.RunFunc`: creates namespaces → installs CRDs → kube-vip → Istio control plane → shared Gateway → Cloudflare Access
     - `namespaces.go` - centralized namespace creation (see convention above)
     - `crds.go` - calls `pkg/crds/{gatewayapi,istio}.InstallCRDs`
     - `providers.go` - builds the Kubernetes provider from the resolved kubeconfig and the Cloudflare provider from `infra.yaml`'s API token

3. **`pkg/crds/`** - generated types + CRD installation, one dir per source:
   - `istio/` - `crds/` (generated from the Istio `base` Helm chart via `crd2pulumi`), `istio-crds.yaml` (the extracted manifest), `gen-crds.sh`, `doc.go` (`//go:generate`), `install.go` (`InstallCRDs`, embeds `istio-crds.yaml`)
   - `gatewayapi/` - same shape, for the Kubernetes Gateway API standard-channel manifest (downloaded directly from GitHub releases, no Helm chart)

4. **`pkg/components/`** - actual deployable components:
   - `kubevip/` - Kube-vip component for control plane HA (`component.go`, `rbac.go`, `daemonset.go`) - targets the pre-existing `kube-system` namespace, doesn't need the namespace-arg convention
   - `istio/` - `component.go` (`NewIstio`: istiod + CNI + ztunnel Helm charts only - ambient mesh control plane)
     - `gateway/` - `NewGateway`: the single shared Kubernetes Gateway API `Gateway`, open to `HTTPRoute`s from any namespace. References istiod's auto-created `GatewayClass` named `istio` by name rather than owning one (avoids the same ownership-conflict class the namespace convention above fixes)
     - `route/` - `NewRoute`: reusable `HTTPRoute` attached to the shared Gateway, for future app deployments to call - not wired into `deploy.Program` yet (nothing to route to)
   - `longhorn/` - `NewLonghorn`: Helm-deployed distributed block storage, exports the default StorageClass, and exposes its UI over Tailscale (`storage.<tailnet>`) the same way `pkg/deploy/applications/private.go` exposes an app - wired into `deploy.Program`
   - `cloudflare/{tunnel,auth}/` - wired into `deploy.Program`

5. **State** (`.pulumi-state/`):
   - Git-crypt'd local file backend (see `.gitattributes`) - `cli/cmd/pulumi/pulumi.go` points the Automation API workspace's backend at `file://<absolute path to .pulumi-state>`, computed relative to the CLI's working directory (repo root)
   - Secrets provider is a blank passphrase (`PULUMI_CONFIG_PASSPHRASE=""`) - protection at rest comes from git-crypt, not a second passphrase to manage

6. **Configuration Files**:
   - `infra.yaml` - Main configuration (VIP, nodes, SSH settings, cluster token; Cloudflare account ID/API token/tunnel domain/Access allowed emails)

**Workflow**:
```bash
# 1. Bootstrap first node
go run cli/main.go bootstrap --node pi-0
go run cli/main.go k3s --node pi-0 --cluster-init

# 2. Deploy kube-vip via Pulumi (connects directly to pi-0 - the VIP doesn't
# exist yet)
go run cli/main.go up

# 3. Merge kubeconfig into your default kubeconfig for kubectl/k9s (now via the VIP)
go run cli/main.go kubeconfig

# 4. Join additional nodes via VIP (cluster token fetched and saved automatically)
go run cli/main.go bootstrap --node pi-1
go run cli/main.go k3s --node pi-1 --server https://192.168.1.50:6443
```

### Legacy TypeScript Infrastructure Stack (being migrated)

1. **Hardware Layer** (`components/infra/`):

    - `raspberrypi5/` - Raspberry Pi 5 configuration and provisioning
    - `k3s/` - Lightweight Kubernetes distribution setup
    - `pki/` - Public Key Infrastructure for certificates

2. **Networking & VPN** (`project/vpn.ts`, `project/network.ts`):

    - Tailscale integration for secure remote access
    - MetalLB for LoadBalancer services
    - DNS configuration with Pi-hole

3. **Kubernetes Applications** (`components/kubernetes/`):
    - `certmanager/` - Automatic TLS certificate management
    - `istio/` - Service mesh with Gateway API
    - `metallb/` - Bare metal load balancer
    - `pihole/` - Network-wide ad blocking and DNS
    - `tailscale/` - VPN operator for Kubernetes
    - `externaldns/` - Automatic DNS record management
    - `homeassistant/` - Home automation platform
    - `prometheus-operator/` - Monitoring and alerting
    - `grafana/` - Visualization and dashboards
    - `node-exporter/` - Node-level metrics collection
    - `kube-state-metrics/` - Kubernetes object metrics
    - `cadvisor/` - Container-level metrics via kubelet
    - `longhorn/` - Distributed block storage

### Key Configuration

- **Main entry point**: `project/index.ts` orchestrates the entire deployment with dependency order (VPN → Cluster → PKI → Storage → Network → DNS → Monitoring → Applications)
- **Configuration**: `project/config.ts` loads node connection details and Tailscale credentials from Pulumi config
- **Version management**: `.versions.ts` centralizes all application versions and uses current Git commit for custom builds
- **CRD Generation**: `.gen.ts` script orchestrates TypeScript binding generation for all Kubernetes operators

### Generated Files

**Go Infrastructure:**
- Kubeconfig - Merged by the CLI into your default kubeconfig (`$KUBECONFIG`, else `~/.kube/config`) under the `homelab` context, for kubectl/k9s/Pulumi access
- `ca.pem` - Root CA certificate for the PKI (gitignored)

**TypeScript Infrastructure:**
- `components/kubernetes/*/crds/` - Generated TypeScript CRD bindings from upstream sources
- `components/kubernetes/*/gen/` - CRD generation scripts that fetch and process upstream CRDs

### Component Architecture

Each Kubernetes component follows a consistent pattern:

1. **Component class** (`index.ts`) - Main component with configuration and dependencies
2. **CRD generation script** (`gen/*.ts`) - Fetches upstream CRDs and generates TypeScript bindings
3. **Generated CRDs** (`crds/`) - TypeScript definitions for Kubernetes Custom Resources
4. **Version pinning** - All versions centralized in `.versions.ts` for consistency

### Monitoring Stack

- **Prometheus Operator** - Manages Prometheus instances and monitoring configuration
- **ServiceMonitors** - Automatic service discovery for metrics scraping
- **Grafana Dashboards** - Located in `project/monitoring/dashboards/` as JSON files
- **Cadvisor Component** - Provides container-level metrics via kubelet ServiceMonitor
- **Dashboard Management** - Dashboards exported via `project/monitoring/dashboards/index.ts`

### Development Workflow

1. Infrastructure changes are made in TypeScript using Pulumi
2. Run `yarn gen` to regenerate any CRD bindings if upstream charts change
3. Use `yarn format` to ensure consistent code style
4. Deploy with `yarn deploy` (requires proper Pulumi configuration)
5. Access cluster with `yarn k9s` or standard kubectl with the generated kubeconfig

The project follows a modular component architecture where each service is self-contained but can depend on shared infrastructure like PKI and networking.

### Important Implementation Details

**Go Infrastructure:**
- **Kube-vip Configuration**: Control plane HA only (`svc_enable=false`), MetalLB handles LoadBalancer services
- **Network Interface**: Hardcoded to `eth0` (Raspberry Pi default)
- **VIP Address**: 192.168.1.50 (configured in `infra.yaml`)
- **Leader Election**: Uses Kubernetes leases for automatic failover
- **Node Affinity**: Only runs on control plane nodes (`node-role.kubernetes.io/control-plane=true`)
- **Security**: Requires NET_ADMIN and NET_RAW capabilities for ARP

**TypeScript Infrastructure:**
- **Pulumi Force Patching**: Deployment uses `PULUMI_K8S_ENABLE_PATCH_FORCE=true` to handle immutable field updates
- **Git-based Versioning**: Custom components use current Git commit SHA for versioning (see `.versions.ts`)
- **CRD Lifecycle**: CRDs are regenerated from upstream sources, not manually maintained
- **Dependency Management**: Services have explicit dependencies enforced through Pulumi resource relationships
- **Resource Sizing**: Prometheus requires adequate memory (2Gi limit) for container metrics collection
- **Grafana Dashboards**: Use kube-state-metrics `exported_*` labels for actual pod information, not scraper pod labels

## Troubleshooting

### Go Infrastructure

**Kube-vip pods not running:**
```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=kube-vip
kubectl logs -n kube-system -l app.kubernetes.io/name=kube-vip
```
Common issues:
- Interface not found: Verify `eth0` exists on your Raspberry Pi
- VIP already in use: Ensure 192.168.1.50 is not assigned to another device
- No control plane nodes: Requires nodes with `node-role.kubernetes.io/control-plane=true`

**VIP not responding:**
```bash
ping 192.168.1.50
kubectl get leases -n kube-system | grep vip
```

**Cluster token not found:**
`k3s --server ...` fetches and saves it to infra.yaml's `cluster.token` automatically; if that fails, pass `--token` explicitly or check SSH connectivity to the join server.

**SSH connection fails:**
Set password via environment variable: `export HOMELAB_SSH_PASSWORD=<password>`

### TypeScript Infrastructure

**`no IP addresses available in range set`:**
`ssh` into the node and run `sudo rm -rf /var/lib/cni/networks/cbr0 && sudo reboot`. See [k3s issue](https://github.com/k3s-io/k3s/issues/4682).

**k3s node fails:**
Check `systemctl status k3s.service` and `journalctl -xeu k3s.service` for details.

**Home Assistant Configuration:**
The `yarn deploy` command automatically extracts Home Assistant configuration before deployment using `components/kubernetes/homeassistant/extract-config.ts`. This processes the YAML configuration files in `components/kubernetes/homeassistant/config/` and makes them available to the deployment.
