import * as path from 'path'
import { versions } from '../../../../.versions'
import { cleanDestinationDirectory, crd2pulumi } from '../../../../libs/gen'

export async function tailscale() {
    const destination = path.join(__dirname, '../crds')
    await cleanDestinationDirectory(destination)
    await crd2pulumi({
        destination,
        sources: [
            `https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${versions.tailscale}/cmd/k8s-operator/deploy/crds/tailscale.com_connectors.yaml`,
            `https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${versions.tailscale}/cmd/k8s-operator/deploy/crds/tailscale.com_dnsconfigs.yaml`,
            `https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${versions.tailscale}/cmd/k8s-operator/deploy/crds/tailscale.com_proxyclasses.yaml`,
            `https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${versions.tailscale}/cmd/k8s-operator/deploy/crds/tailscale.com_proxygroups.yaml`,
            `https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${versions.tailscale}/cmd/k8s-operator/deploy/crds/tailscale.com_recorders.yaml`,
        ],
    })
}

// #!/bin/bash
//
// VERSION = 1.78.3
// DIR = "$( cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd )"
//
// # Generate strongly typed code for working with the custom resources
// rm - rf "${DIR}/crds"
// mkdir - p "${DIR}/crds"
// crd2pulumi--nodejsPath "$DIR/crds" --force \
// "https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${VERSION}/cmd/k8s-operator/deploy/crds/tailscale.com_connectors.yaml" \
// "https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${VERSION}/cmd/k8s-operator/deploy/crds/tailscale.com_dnsconfigs.yaml" \
// "https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${VERSION}/cmd/k8s-operator/deploy/crds/tailscale.com_proxyclasses.yaml" \
// "https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${VERSION}/cmd/k8s-operator/deploy/crds/tailscale.com_proxygroups.yaml" \
// "https://raw.githubusercontent.com/tailscale/tailscale/refs/tags/v${VERSION}/cmd/k8s-operator/deploy/crds/tailscale.com_recorders.yaml" \
//
// rm "${DIR}/crds/tsconfig.json" "${DIR}/crds/README.md" "${DIR}/crds/package.json"
//
