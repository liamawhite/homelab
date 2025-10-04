import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { ServiceMonitor } from '../prometheus-operator/crds/monitoring/v1'

export class Cadvisor extends pulumi.ComponentResource {
    private args: CadvisorArgs
    private opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }

    constructor(
        name: string,
        args: CadvisorArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:cadvisor', name, {}, opts)
        this.args = args
        this.opts = opts

        this.registerOutputs({})
    }

    createServiceMonitor(): ServiceMonitor {
        const localOpts = { ...this.opts, parent: this }

        return new ServiceMonitor(
            'cadvisor-sm',
            {
                metadata: {
                    name: 'cadvisor',
                    namespace: this.args.namespace,
                    labels: {
                        'app.kubernetes.io/name': 'cadvisor',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                    },
                },
                spec: {
                    endpoints: [
                        {
                            bearerTokenFile: '/var/run/secrets/kubernetes.io/serviceaccount/token',
                            honorLabels: true,
                            interval: '30s',
                            port: 'https-metrics',
                            scheme: 'https',
                            tlsConfig: {
                                insecureSkipVerify: true,
                            },
                            relabelings: [
                                {
                                    action: 'labelmap',
                                    regex: '__meta_kubernetes_node_label_(.+)',
                                },
                            ],
                            metricRelabelings: [
                                {
                                    action: 'drop',
                                    regex: 'container_([a-z_]+_seconds_total|cpu_(cfs_throttled|periods_total)|tasks_state|memory_failures_total|spec_.*)',
                                    sourceLabels: ['__name__'],
                                },
                            ],
                        },
                        {
                            bearerTokenFile: '/var/run/secrets/kubernetes.io/serviceaccount/token',
                            honorLabels: true,
                            interval: '30s',
                            path: '/metrics/cadvisor',
                            port: 'https-metrics',
                            scheme: 'https',
                            tlsConfig: {
                                insecureSkipVerify: true,
                            },
                            relabelings: [
                                {
                                    action: 'labelmap',
                                    regex: '__meta_kubernetes_node_label_(.+)',
                                },
                            ],
                            metricRelabelings: [
                                {
                                    action: 'drop',
                                    regex: 'container_(network_tcp_usage_total|network_udp_usage_total|tasks_state|cpu_load_average_10s)',
                                    sourceLabels: ['__name__'],
                                },
                            ],
                        },
                    ],
                    jobLabel: 'app.kubernetes.io/name',
                    namespaceSelector: {
                        matchNames: ['kube-system'],
                    },
                    selector: {
                        matchLabels: {
                            'app.kubernetes.io/name': 'kubelet',
                        },
                    },
                },
            },
            localOpts,
        )
    }
}

export interface CadvisorArgs {
    namespace: pulumi.Input<string>
}
