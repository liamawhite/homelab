#!/bin/bash
# Generate Istio CRD types from Helm chart
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ISTIO_VERSION="1.28.2"

echo "Updating Istio Helm repository..."
helm repo add istio https://istio-release.storage.googleapis.com/charts 2>/dev/null || true
helm repo update istio

echo "Extracting Istio CRDs from Helm chart version ${ISTIO_VERSION}..."
helm template istio-base istio/base \
  --version "${ISTIO_VERSION}" \
  --include-crds > "${SCRIPT_DIR}/istio-crds.yaml"

echo "Generating Go types from CRDs..."
cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds -f istio-crds.yaml

echo "Successfully generated Istio CRD types!"
