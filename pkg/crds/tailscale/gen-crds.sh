#!/bin/bash
# Generate Tailscale CRD types from the tailscale-operator Helm chart.
#
# Unlike pkg/crds/istio and pkg/crds/gatewayapi, this package has no
# install.go - the tailscale-operator chart's own installCRDs value (true by
# default) already installs these CRDs as part of its own release (see
# pkg/components/tailscale), the same single-chart pattern pkg/crds/cilium
# documents. Unlike Istio's "base" chart, the tailscale-operator chart has no
# Helm-recognized crds/ directory - its CRDs are plain templates gated by
# .Values.installCRDs, mixed in with the chart's other resources (Deployment,
# RBAC, IngressClass), so `helm template` output has to be filtered down to
# just the CustomResourceDefinition documents.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TAILSCALE_VERSION="$("${SCRIPT_DIR}/../../versions/version.sh" Tailscale)"

echo "Updating Tailscale Helm repository..."
helm repo add tailscale https://pkgs.tailscale.com/helmcharts 2>/dev/null || true
helm repo update tailscale

echo "Extracting Tailscale CRDs from Helm chart version ${TAILSCALE_VERSION}..."
helm template tailscale-operator tailscale/tailscale-operator \
  --version "${TAILSCALE_VERSION}" --namespace tailscale \
  | yq 'select(.kind == "CustomResourceDefinition")' - \
  > "${SCRIPT_DIR}/tailscale-crds.yaml"

echo "Generating Go types from CRDs..."
cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds -f tailscale-crds.yaml

echo "Successfully generated Tailscale CRD types!"
