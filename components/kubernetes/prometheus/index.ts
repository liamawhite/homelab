import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { Gateway } from '../gateway'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'
import { Prometheus } from '../prometheus-operator/crds/monitoring/v1'
import { versions } from '../../../.versions'

export class PrometheusInstance extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: PrometheusArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:prometheus', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // ServiceAccount for Prometheus
        const serviceAccount = new k8s.core.v1.ServiceAccount(
            'prometheus-sa',
            {
                metadata: {
                    name: 'prometheus-k8s',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'prometheus',
                        'app.kubernetes.io/instance': 'k8s',
                        'app.kubernetes.io/name': 'prometheus',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheus,
                    },
                },
            },
            localOpts,
        )

        // ClusterRole for Prometheus
        const clusterRole = new k8s.rbac.v1.ClusterRole(
            'prometheus-cr',
            {
                metadata: {
                    name: 'prometheus-k8s',
                    labels: {
                        'app.kubernetes.io/component': 'prometheus',
                        'app.kubernetes.io/instance': 'k8s',
                        'app.kubernetes.io/name': 'prometheus',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheus,
                    },
                },
                rules: [
                    {
                        apiGroups: [''],
                        resources: ['nodes', 'nodes/metrics', 'services', 'endpoints', 'pods'],
                        verbs: ['get', 'list', 'watch'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['configmaps'],
                        verbs: ['get'],
                    },
                    {
                        apiGroups: ['networking.k8s.io'],
                        resources: ['ingresses'],
                        verbs: ['get', 'list', 'watch'],
                    },
                    {
                        nonResourceURLs: ['/metrics', '/metrics/slis'],
                        verbs: ['get'],
                    },
                ],
            },
            localOpts,
        )

        // ClusterRoleBinding for Prometheus
        const clusterRoleBinding = new k8s.rbac.v1.ClusterRoleBinding(
            'prometheus-crb',
            {
                metadata: {
                    name: 'prometheus-k8s',
                    labels: {
                        'app.kubernetes.io/component': 'prometheus',
                        'app.kubernetes.io/instance': 'k8s',
                        'app.kubernetes.io/name': 'prometheus',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheus,
                    },
                },
                roleRef: {
                    apiGroup: 'rbac.authorization.k8s.io',
                    kind: 'ClusterRole',
                    name: clusterRole.metadata.name,
                },
                subjects: [
                    {
                        kind: 'ServiceAccount',
                        name: serviceAccount.metadata.name,
                        namespace: args.namespace,
                    },
                ],
            },
            localOpts,
        )

        // Prometheus Custom Resource
        const prometheus = new Prometheus(
            'prometheus',
            {
                metadata: {
                    name: 'k8s',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'prometheus',
                        'app.kubernetes.io/instance': 'k8s',
                        'app.kubernetes.io/name': 'prometheus',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheus,
                    },
                },
                spec: {
                    image: `quay.io/prometheus/prometheus:v${versions.prometheus}`,
                    version: versions.prometheus,
                    replicas: 1,
                    retention: '30d',
                    serviceAccountName: serviceAccount.metadata.name,
                    securityContext: {
                        fsGroup: 2000,
                        runAsNonRoot: true,
                        runAsUser: 1000,
                    },
                    resources: {
                        requests: {
                            memory: '64Mi',
                            cpu: '10m',
                        },
                        limits: {
                            memory: '128Mi',
                            cpu: '100m',
                        },
                    },
                    storage: {
                        volumeClaimTemplate: {
                            spec: {
                                storageClassName: args.storage.storageClassName,
                                accessModes: ['ReadWriteOnce'],
                                resources: {
                                    requests: {
                                        storage: args.storage.size,
                                    },
                                },
                            },
                        },
                    },
                    serviceMonitorNamespaceSelector: {},
                    serviceMonitorSelector: {},
                    podMonitorNamespaceSelector: {},
                    podMonitorSelector: {},
                    ruleNamespaceSelector: {},
                    ruleSelector: {},
                    probeNamespaceSelector: {},
                    probeSelector: {},
                    scrapeConfigNamespaceSelector: {},
                    scrapeConfigSelector: {},
                    nodeSelector: {
                        'kubernetes.io/os': 'linux',
                    },
                    podMetadata: {
                        labels: {
                            'app.kubernetes.io/component': 'prometheus',
                            'app.kubernetes.io/instance': 'k8s',
                            'app.kubernetes.io/name': 'prometheus',
                            'app.kubernetes.io/part-of': 'kube-prometheus',
                            'app.kubernetes.io/version': versions.prometheus,
                        },
                    },
                    enableFeatures: [],
                    externalLabels: {},
                },
            },
            { ...localOpts, dependsOn: [serviceAccount, clusterRole, clusterRoleBinding] },
        )

        // Service for Prometheus web UI
        const service = new k8s.core.v1.Service(
            'prometheus-service',
            {
                metadata: {
                    name: 'prometheus-k8s',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'prometheus',
                        'app.kubernetes.io/instance': 'k8s',
                        'app.kubernetes.io/name': 'prometheus',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheus,
                    },
                },
                spec: {
                    ports: [
                        {
                            name: 'web',
                            port: 9090,
                            targetPort: 'web',
                        },
                        {
                            name: 'reloader-web',
                            port: 8080,
                            targetPort: 'reloader-web',
                        },
                    ],
                    selector: {
                        prometheus: 'k8s',
                    },
                    sessionAffinity: 'ClientIP',
                },
            },
            localOpts,
        )

        // Gateway for web access
        const gateway = new Gateway(
            'prometheus-gateway',
            {
                namespace: args.namespace,
                hostname: args.web.hostname,
                serviceName: service.metadata.name,
                servicePort: 9090,
                issuer: args.web.issuer,
                tailscale: args.web.tailscale,
            },
            localOpts,
        )

        this.registerOutputs({
            serviceAccount,
            clusterRole,
            clusterRoleBinding,
            prometheus,
            service,
            gateway,
        })
    }
}

export interface PrometheusArgs {
    namespace: pulumi.Input<string>
    storage: {
        size: string
        storageClassName: pulumi.Input<string>
    }
    web: {
        hostname: string
        issuer: certmanager.v1.CertificateSpecIssuerRef
        tailscale?: {
            enabled: boolean
            hostname?: string
        }
    }
}
