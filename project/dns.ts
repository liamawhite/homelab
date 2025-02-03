import { loadConfig } from './config'
import { Pihole } from '../components/kubernetes/pihole'
import { ExternalDns } from '../components/kubernetes/externaldns'
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

    const pihole = new Pihole(
        'pihole',
        {
            web: { hostname: 'pihole.homelab', issuer: pki.issuer.issuerRef() },
            dns: {
                annotations: {
                    'metallb.universe.tf/address-pool': network.dnsPool.metadata.name,
                },
            },
        },
        opts,
    )

    const externalDns = new ExternalDns(
        'external-dns',
        {
            pihole: {
                web: pihole.localAddress,
                password: pihole.password,
            },
        },
        opts,
    )

    return {
        pihole: {
            password: pihole.password,
        },
        ready: [pihole, externalDns],
    }
}
