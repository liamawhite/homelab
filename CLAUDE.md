# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

The project uses Nix for development environment management and Yarn for package management:

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

This is a Pulumi-based Infrastructure as Code project that deploys a complete homelab on Raspberry Pi hardware.

### Core Infrastructure Stack

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

- `kubeconfig` - Generated at repo root for kubectl/k9s access (gitignored)
- `ca.pem` - Root CA certificate for the PKI (gitignored)
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

- **Pulumi Force Patching**: Deployment uses `PULUMI_K8S_ENABLE_PATCH_FORCE=true` to handle immutable field updates
- **Git-based Versioning**: Custom components use current Git commit SHA for versioning (see `.versions.ts`)
- **CRD Lifecycle**: CRDs are regenerated from upstream sources, not manually maintained
- **Dependency Management**: Services have explicit dependencies enforced through Pulumi resource relationships
- **Resource Sizing**: Prometheus requires adequate memory (2Gi limit) for container metrics collection
- **Grafana Dashboards**: Use kube-state-metrics `exported_*` labels for actual pod information, not scraper pod labels

## Troubleshooting

### `no IP addresses available in range set`

`ssh` into the node and run `sudo rm -rf /var/lib/cni/networks/cbr0 && sudo reboot`. See [k3s issue](https://github.com/k3s-io/k3s/issues/4682).

### k3s node fails

Check `systemctl status k3s.service` and `journalctl -xeu k3s.service` for details.

### Home Assistant Configuration

The `yarn deploy` command automatically extracts Home Assistant configuration before deployment using `components/kubernetes/homeassistant/extract-config.ts`. This processes the YAML configuration files in `components/kubernetes/homeassistant/config/` and makes them available to the deployment.
