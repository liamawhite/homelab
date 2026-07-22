#!/bin/bash
# Generate the Light/HueBridge CRDs' DeepCopy methods, CRD manifests, and
# Pulumi Go SDK types.
#
# Unlike this directory's sibling gen-crds.sh scripts (whose CRD manifest
# comes from an upstream Helm chart or GitHub release), the manifest here
# is generated straight from this repo's own annotated Go structs in
# applications/lights-controller/api/v1alpha1, via controller-gen
# (sigs.k8s.io/controller-tools) - see doc.go for why. Once that manifest
# exists, though, this generates typed Pulumi Go bindings from it via
# crd2pulumi exactly like every sibling package, so callers create Light/
# HueBridge instances the same way they create e.g.
# CiliumClusterwideNetworkPolicy instances - through a typed NewX
# constructor - rather than the untyped apiextensions.CustomResource
# escape hatch.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROLLER_MODULE_DIR="$(cd "${SCRIPT_DIR}/../../../applications/lights-controller" && pwd)"

echo "Generating DeepCopy methods..."
(cd "${CONTROLLER_MODULE_DIR}" && controller-gen object paths=./api/...)

echo "Generating CRD manifests..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
(cd "${CONTROLLER_MODULE_DIR}" && controller-gen crd paths=./api/... output:crd:dir="${TMP_DIR}")

# controller-gen names each output "<group>_<plural>.yaml" - one file per
# Kind found in ./api/..., not the single fixed name this script used to
# assume. Map each to the stable filename install.go embeds.
mv "${TMP_DIR}/lights.homelab.internal_lights.yaml" "${SCRIPT_DIR}/light-crd.yaml"
mv "${TMP_DIR}/lights.homelab.internal_huebridges.yaml" "${SCRIPT_DIR}/huebridge-crd.yaml"
mv "${TMP_DIR}/lights.homelab.internal_switches.yaml" "${SCRIPT_DIR}/switch-crd.yaml"

echo "Generating Pulumi Go types from CRDs..."
(cd "${SCRIPT_DIR}" && crd2pulumi --goPath crds --goName crds -f light-crd.yaml huebridge-crd.yaml switch-crd.yaml)

echo "Successfully generated Light/HueBridge CRD types!"
