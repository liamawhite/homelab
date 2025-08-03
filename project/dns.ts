import { loadConfig } from './config'
import { CoreDns } from '../components/kubernetes/coredns'
import { K8sGateway } from '../components/kubernetes/k8sgateway'
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

    // Deploy k8s-gateway for internal .homelab domain resolution
    const k8sGateway = new K8sGateway(
        'k8s-gateway',
        {
            namespace: dnsNamespace.metadata.name,
            domain: 'homelab',
            clusterIP: '10.43.200.53',
        },
        opts,
    )

    // External CoreDNS with .homelab forwarding to k8s-gateway
    const coredns = new CoreDns(
        'coredns',
        {
            namespace: dnsNamespace.metadata.name,
            dns: {
                annotations: {
                    'metallb.universe.tf/address-pool': network.dnsPool.metadata.name,
                },
            },
            homelabTLDForwarder: k8sGateway.serviceIP,
        },
        opts,
    )

    return {
        ready: [k8sGateway, coredns],
    }
}
