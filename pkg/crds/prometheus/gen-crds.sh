#!/bin/bash
# Generate prometheus-operator CRD types from the upstream release bundle.
#
# Unlike gatewayapi's gen-crds.sh (whose downloaded manifest is CRDs-only
# already), prometheus-operator's release bundle.yaml is its entire install
# manifest - CRDs plus the Namespace/ServiceAccount/ClusterRole/
# ClusterRoleBinding/Service/Deployment for the operator itself. This repo's
# own pkg/components/prometheus hand-writes that Deployment/RBAC instead of
# taking upstream's copy, so this script filters the bundle down to just the
# CustomResourceDefinition documents before handing it to crd2pulumi.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMETHEUS_OPERATOR_VERSION="$("${SCRIPT_DIR}/../../versions/version.sh" PrometheusOperator)"

echo "Downloading prometheus-operator bundle version ${PROMETHEUS_OPERATOR_VERSION}..."
curl -sL --fail \
  "https://github.com/prometheus-operator/prometheus-operator/releases/download/v${PROMETHEUS_OPERATOR_VERSION}/bundle.yaml" \
  -o "${SCRIPT_DIR}/bundle-temp.yaml"

echo "Filtering to CustomResourceDefinition documents..."
yq eval-all 'select(.kind == "CustomResourceDefinition")' \
  "${SCRIPT_DIR}/bundle-temp.yaml" > "${SCRIPT_DIR}/prometheus-operator-crds.yaml"
rm "${SCRIPT_DIR}/bundle-temp.yaml"

echo "Generating Go types from CRDs..."
cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds -f prometheus-operator-crds.yaml

echo "Successfully generated prometheus-operator CRD types!"
