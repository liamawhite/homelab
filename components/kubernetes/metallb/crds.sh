#!/bin/bash

VERSION=0.14.9
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
TMP_DIR=$(mktemp -d -t metallb-XXXXXXXXXX)

# Retrieve the CRDs from the Metallb GitHub repository
wget -qO- "https://github.com/metallb/metallb/releases/download/metallb-chart-${VERSION}/metallb-${VERSION}.tgz" | tar -xvz -C "${TMP_DIR}"

# CRDs are in the charts/crds subchart
helm template crdgen "${TMP_DIR}/metallb/charts/crds" > "${TMP_DIR}/crds.yaml"

# Generate strongly typed code for working with the custom resources
rm -rf "${DIR}/crds"
mkdir -p "${DIR}/crds"
crd2pulumi --nodejsPath "$DIR/crds" --force "${TMP_DIR}/crds.yaml"
rm "${DIR}/crds/tsconfig.json" "${DIR}/crds/README.md" "${DIR}/crds/package.json"

