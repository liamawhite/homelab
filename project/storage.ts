import { Longhorn } from '../components/kubernetes/longhorn'
import { configureCluster } from './cluster'
import { configurePki } from './pki'

export function configureStorage({
    cluster,
    pki,
}: {
    cluster: ReturnType<typeof configureCluster>
    pki: ReturnType<typeof configurePki>
}) {
    const opts = { provider: cluster.provider }

    const longhorn = new Longhorn(
        'longhorn',
        {
            web: {
                hostname: 'storage.homelab',
                issuer: pki.issuer.issuerRef(),
            },
        },
        opts,
    )

    return {
        longhorn,
        ready: [longhorn],
    }
}
