import * as k8s from '@pulumi/kubernetes'
import { PrometheusOperator } from '../../components/kubernetes/prometheus-operator'
import { PrometheusInstance } from '../../components/kubernetes/prometheus'
import { NodeExporter } from '../../components/kubernetes/node-exporter'
import { KubeStateMetrics } from '../../components/kubernetes/kube-state-metrics'
import { Grafana } from '../../components/kubernetes/grafana'
import { Cadvisor } from '../../components/kubernetes/cadvisor'
import { dashboards } from './dashboards'
import { configureCluster } from '../cluster'
import { configurePki } from '../pki'
import { configureStorage } from '../storage'

export function configureMonitoring({
    provider,
    pki,
    storage,
}: ReturnType<typeof configureCluster> & {
    pki: ReturnType<typeof configurePki>
    storage: ReturnType<typeof configureStorage>
}) {
    const opts = { provider, dependsOn: [...pki.ready, ...storage.ready] }

    const namespace = new k8s.core.v1.Namespace(
        'monitoring-ns',
        {
            metadata: { name: 'monitoring' },
        },
        { provider },
    )

    const prometheusOperator = new PrometheusOperator(
        'prometheus-operator',
        { namespace: namespace.metadata.name },
        { provider },
    )

    const prometheus = new PrometheusInstance(
        'prometheus',
        {
            namespace: namespace.metadata.name,
            storage: {
                size: '50Gi',
                storageClassName: storage.defaultStorageClass.metadata.name,
            },
            web: {
                hostname: 'prometheus.homelab',
                issuer: pki.issuer.issuerRef(),
                tailscale: {
                    enabled: true,
                    hostname: 'prometheus',
                },
            },
        },
        { ...opts, dependsOn: [prometheusOperator] },
    )

    const nodeExporter = new NodeExporter(
        'node-exporter',
        {
            namespace: namespace.metadata.name,
        },
        { ...opts, dependsOn: [prometheusOperator] },
    )

    nodeExporter.createServiceMonitor()

    const kubeStateMetrics = new KubeStateMetrics(
        'kube-state-metrics',
        {
            namespace: namespace.metadata.name,
        },
        opts,
    )

    kubeStateMetrics.createServiceMonitor()

    const cadvisor = new Cadvisor(
        'cadvisor',
        {
            namespace: namespace.metadata.name,
        },
        { ...opts, dependsOn: [prometheusOperator] },
    )

    cadvisor.createServiceMonitor()

    const grafana = new Grafana(
        'grafana',
        {
            namespace: namespace.metadata.name,
            dashboards: dashboards,
            storage: {
                size: '10Gi',
                storageClassName: storage.defaultStorageClass.metadata.name,
            },
            prometheus: {
                namespace: namespace.metadata.name,
            },
            web: {
                hostname: 'grafana.homelab',
                issuer: pki.issuer.issuerRef(),
                tailscale: {
                    enabled: true,
                    hostname: 'grafana',
                },
            },
        },
        { ...opts, dependsOn: [prometheus] },
    )

    return {
        prometheusOperator,
        prometheus,
        nodeExporter,
        kubeStateMetrics,
        cadvisor,
        grafana,
        ready: [prometheusOperator, prometheus, nodeExporter, kubeStateMetrics, cadvisor, grafana],
    }
}
