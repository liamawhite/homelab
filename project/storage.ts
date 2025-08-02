import { Longhorn } from '../components/kubernetes/longhorn'
import { configureCluster } from './cluster'

export function configureStorage({ cluster }: { cluster: ReturnType<typeof configureCluster> }) {
    const opts = { provider: cluster.provider }

    const longhorn = new Longhorn('longhorn', {}, opts)

    return {
        longhorn,
        ready: [longhorn],
    }
}
