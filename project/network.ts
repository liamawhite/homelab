import { Istio } from '../components/kubernetes/istio'
import { MetalLb } from '../components/kubernetes/metallb'
import {
    IPAddressPool,
    L2Advertisement,
} from '../components/kubernetes/metallb/crds/metallb/v1beta1'
import { TailscaleOperator } from '../components/kubernetes/tailscale'
import { configureCluster } from './cluster'
import { loadConfig } from './config'

export function configureNetwork({
    cluster,
    tailscale,
}: { cluster: ReturnType<typeof configureCluster> } & ReturnType<typeof loadConfig>) {
    const opts = { provider: cluster.provider }

    const metallb = new MetalLb('metallb', {}, opts)
    const ipPool = new IPAddressPool(
        'default-pool',
        {
            metadata: { name: 'homelab', namespace: metallb.namespace },
            spec: {
                addresses: ['192.168.2.2-192.168.2.254'],
            },
        },
        { ...opts, parent: metallb },
    )
    const dnsPool = new IPAddressPool(
        'dns-pool',
        {
            metadata: { name: 'dns', namespace: metallb.namespace },
            spec: {
                addresses: ['192.168.2.1/32'],
                autoAssign: false, // dont auto assign, pihole needs to opt in via annotation
            },
        },
        { ...opts, parent: metallb },
    )

    const advertisment = new L2Advertisement(
        'advertisement',
        {
            metadata: { name: 'homelab', namespace: metallb.namespace },
            spec: {
                ipAddressPools: [ipPool.metadata.name, dnsPool.metadata.name],
            },
        },
        { ...opts, parent: metallb },
    )

    const tailscaleOperator = new TailscaleOperator(
        'tailscale',
        {
            hostname: 'homelab-operator',
            clientId: tailscale.operator.clientId,
            clientSecret: tailscale.operator.clientSecret,
        },
        opts,
    )

    const istio = new Istio('istio', {}, opts)

    return {
        dnsPool: dnsPool,
        ready: [metallb, ipPool, dnsPool, advertisment, tailscaleOperator, istio],
    }
}
