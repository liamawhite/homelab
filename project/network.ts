import { MetalLb } from "../components/metallb"
import { IPAddressPool, L2Advertisement } from "../components/metallb/crds/metallb/v1beta1";
import { TailscaleOperator } from "../components/tailscale";
import { configureCluster } from "./cluster";
import { loadConfig } from "./config";

export function configureNetwork({
    cluster,
    tailscale,
}: { cluster: ReturnType<typeof configureCluster> } & ReturnType<typeof loadConfig>) {
    const opts = { provider: cluster.provider }

    const metallb = new MetalLb('metallb', {}, opts)
    const ipPool = new IPAddressPool('default-pool', {
        metadata: { name: 'homelab', namespace: metallb.namespace },
        spec: {
            addresses: ['192.168.100.6-192.168.100.254'],
        },
    }, { ...opts, parent: metallb })

    const advertisment = new L2Advertisement('advertisement', {
        metadata: { name: 'homelab', namespace: metallb.namespace },
        spec: {
            ipAddressPools: [ipPool.metadata.name],
        },
    }, { ...opts, parent: metallb })

    const tailscaleOperator = new TailscaleOperator('tailscale', {
        hostname: 'homelab-operator',
        clientId: tailscale.operator.clientId,
        clientSecret: tailscale.operator.clientSecret,
    }, opts)


    return { ready: [metallb, ipPool, advertisment] }
}
