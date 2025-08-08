import * as k8s from '@pulumi/kubernetes'
import { PrometheusOperator } from '../components/kubernetes/prometheus-operator'
import { configureCluster } from './cluster'

export function configureMonitoring({ provider }: ReturnType<typeof configureCluster>) {
    const namespace = new k8s.core.v1.Namespace(
        'monitoring-ns',
        {
            metadata: { name: 'monitoring' },
        },
        { provider }
    )

    const prometheusOperator = new PrometheusOperator(
        'prometheus-operator', 
        { namespace: namespace.metadata.name }, 
        { provider }
    )

    return {
        prometheusOperator,
        ready: [prometheusOperator],
    }
}
