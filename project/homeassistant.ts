import * as k8s from '@pulumi/kubernetes'
import { HomeAssistant } from '../components/kubernetes/homeassistant'
import { configureCluster } from './cluster'
import { configurePki } from './pki'
import { configureStorage } from './storage'
import { versions } from '../.versions'
import * as labels from '../components/kubernetes/istio/labels'

export function configureHomeAssistant({
    cluster,
    pki,
    storage,
}: {
    cluster: ReturnType<typeof configureCluster>
    pki: ReturnType<typeof configurePki>
    storage: ReturnType<typeof configureStorage>
}) {
    const opts = { provider: cluster.provider, dependsOn: storage.ready }

    const namespace = new k8s.core.v1.Namespace(
        'homeassistant-namespace',
        {
            metadata: {
                name: 'homeassistant',
                labels: labels.namespace.enableAmbient,
            },
        },
        opts,
    )

    const homeassistant = new HomeAssistant(
        'homeassistant',
        {
            namespace: namespace.metadata.name,
            version: versions.homeassistant,
            image: `ghcr.io/liamawhite/homeassistant:${versions.homeassistant}`, // Auto-built image with integrations
            storage: {
                size: '10Gi',
                storageClassName: storage.defaultStorageClass.metadata.name,
            },
            web: {
                hostname: 'homeassistant.homelab',
                issuer: pki.issuer.issuerRef(),
            },
        },
        { ...opts, dependsOn: [namespace, ...storage.ready] },
    )

    return {
        namespace,
        homeassistant,
        ready: [homeassistant],
    }
}
