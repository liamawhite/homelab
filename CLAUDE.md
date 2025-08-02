# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

The project uses Nix for development environment management and Yarn for package management:

- `nix develop` - Enter the development shell with all required tools
- `yarn check` - Run code generation and formatting (equivalent to `yarn gen && yarn format`)
- `yarn gen` - Generate Kubernetes CRD TypeScript bindings from upstream sources
- `yarn format` - Format all code using Prettier
- `yarn deploy` - Deploy the homelab infrastructure using Pulumi
- `yarn passwords` - Extract and display secrets from the Pulumi stack
- `yarn k9s` - Launch k9s Kubernetes dashboard using the generated kubeconfig

All commands should be run within the Nix development shell (`nix develop`) which provides:

- crd2pulumi (for generating TypeScript from Kubernetes CRDs)
- kubernetes-helm, k9s, jq, git, nodejs_23

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

### Key Configuration

- **Main entry point**: `project/index.ts` orchestrates the entire deployment
- **Configuration**: `project/config.ts` loads node connection details and Tailscale credentials from Pulumi config
- **CRD Generation**: `.gen.ts` script generates TypeScript bindings for Kubernetes Custom Resource Definitions

### Generated Files

- `kubeconfig` - Generated at repo root for kubectl/k9s access
- `ca.pem` - Root CA certificate for the PKI
- `components/kubernetes/*/crds/` - Generated TypeScript CRD bindings
- `components/kubernetes/*/gen/` - Generated CRD helper functions

### Development Workflow

1. Infrastructure changes are made in TypeScript using Pulumi
2. Run `yarn gen` to regenerate any CRD bindings if upstream charts change
3. Use `yarn format` to ensure consistent code style
4. Deploy with `yarn up` (requires proper Pulumi configuration)
5. Access cluster with `yarn k9s` or standard kubectl with the generated kubeconfig

The project follows a modular component architecture where each service is self-contained but can depend on shared infrastructure like PKI and networking.
