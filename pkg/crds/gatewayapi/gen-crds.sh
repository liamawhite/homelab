#!/bin/bash
# Generate Kubernetes Gateway API CRD types from the upstream standard-channel
# release manifest.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_API_VERSION="$("${SCRIPT_DIR}/../../versions/version.sh" GatewayAPI)"

echo "Downloading Gateway API CRDs version ${GATEWAY_API_VERSION}..."
curl -sL --fail \
  "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml" \
  -o "${SCRIPT_DIR}/gateway-api-crds.yaml"

echo "Generating Go types from CRDs..."
cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds -f gateway-api-crds.yaml

echo "Successfully generated Gateway API CRD types!"
