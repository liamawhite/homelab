#!/bin/bash
# Generate Cilium CRD types from the upstream release manifests.
#
# Unlike pkg/crds/istio and pkg/crds/gatewayapi, this package has no
# install.go - Cilium's own Helm chart (pkg/components/cilium) already
# installs its CRDs as part of the agent/operator deployment. This package
# exists purely to give pkg/components/cilium typed Go structs to write
# NetworkPolicy/ClusterwideNetworkPolicy resources with.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CILIUM_VERSION="$("${SCRIPT_DIR}/../../versions/version.sh" Cilium)"

echo "Downloading Cilium CRDs version ${CILIUM_VERSION}..."
curl -sL --fail \
  "https://raw.githubusercontent.com/cilium/cilium/v${CILIUM_VERSION}/pkg/k8s/apis/cilium.io/client/crds/v2/ciliumnetworkpolicies.yaml" \
  -o "${SCRIPT_DIR}/ciliumnetworkpolicies.yaml"
curl -sL --fail \
  "https://raw.githubusercontent.com/cilium/cilium/v${CILIUM_VERSION}/pkg/k8s/apis/cilium.io/client/crds/v2/ciliumclusterwidenetworkpolicies.yaml" \
  -o "${SCRIPT_DIR}/ciliumclusterwidenetworkpolicies.yaml"

echo "Generating Go types from CRDs..."
cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds \
  -f ciliumnetworkpolicies.yaml \
  -f ciliumclusterwidenetworkpolicies.yaml

echo "Successfully generated Cilium CRD types!"
