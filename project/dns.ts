import { loadConfig } from './config'
import { CoreDns } from '../components/kubernetes/coredns'
import { configureCluster } from './cluster'
import { configureNetwork } from './network'
import { configurePki } from './pki'

export function configureDns({
    cluster,
    pki,
    network,
}: {
    cluster: ReturnType<typeof configureCluster>
    pki: ReturnType<typeof configurePki>
    network: ReturnType<typeof configureNetwork>
} & ReturnType<typeof loadConfig>) {
    const opts = { provider: cluster.provider, dependsOn: network.ready }

    const coredns = new CoreDns(
        'coredns',
        {
            dns: {
                annotations: {
                    'metallb.universe.tf/address-pool': network.dnsPool.metadata.name,
                },
            },
        },
        opts,
    )

    return {
        coredns: {
            localAddress: coredns.localAddress,
        },
        ready: [coredns],
    }
}
