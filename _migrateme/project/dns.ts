import { loadConfig } from './config'
import { CoreDns } from '../components/kubernetes/coredns'
import { configureCluster } from './cluster'
import { configureNetwork } from './network'
import * as k8s from '@pulumi/kubernetes'
import * as labels from '../components/kubernetes/istio/labels'

export function configureDns({
    cluster,
    network,
}: {
    cluster: ReturnType<typeof configureCluster>
    network: ReturnType<typeof configureNetwork>
} & ReturnType<typeof loadConfig>) {
    const opts = { provider: cluster.provider, dependsOn: network.ready }

    // Create shared DNS namespace
    const dnsNamespace = new k8s.core.v1.Namespace(
        'home-dns',
        {
            metadata: {
                name: 'home-dns',
                labels: labels.namespace.enableAmbient,
            },
        },
        opts,
    )

    // External CoreDNS with built-in k8s_gateway plugin
    const coredns = new CoreDns(
        'coredns',
        {
            namespace: dnsNamespace.metadata.name,
            dns: {
                annotations: {
                    'metallb.universe.tf/address-pool': network.dnsPool.metadata.name,
                },
            },
            homelabTLDForwarder: '127.0.0.1',
        },
        opts,
    )

    return {
        ready: [coredns],
    }
}
