import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { ServiceMonitor } from '../prometheus-operator/crds/monitoring/v1'
import { versions } from '../../../.versions'

export class KubeStateMetrics extends pulumi.ComponentResource {
    public readonly deployment: k8s.apps.v1.Deployment
    public readonly service: k8s.core.v1.Service
    public readonly serviceAccount: k8s.core.v1.ServiceAccount
    public readonly clusterRole: k8s.rbac.v1.ClusterRole
    public readonly clusterRoleBinding: k8s.rbac.v1.ClusterRoleBinding

    constructor(
        name: string,
        args: KubeStateMetricsArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:kube-state-metrics', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // ServiceAccount
        this.serviceAccount = new k8s.core.v1.ServiceAccount(
            `${name}-sa`,
            {
                metadata: {
                    name: 'kube-state-metrics',
                    namespace: args.namespace,
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
            },
            localOpts,
        )

        // ClusterRole with permissions to read cluster state
        this.clusterRole = new k8s.rbac.v1.ClusterRole(
            `${name}-cluster-role`,
            {
                metadata: {
                    name: 'kube-state-metrics',
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
                rules: [
                    {
                        apiGroups: [''],
                        resources: [
                            'configmaps',
                            'secrets',
                            'nodes',
                            'pods',
                            'services',
                            'serviceaccounts',
                            'resourcequotas',
                            'replicationcontrollers',
                            'limitranges',
                            'persistentvolumeclaims',
                            'persistentvolumes',
                            'namespaces',
                            'endpoints',
                        ],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['apps'],
                        resources: ['statefulsets', 'daemonsets', 'deployments', 'replicasets'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['batch'],
                        resources: ['cronjobs', 'jobs'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['autoscaling'],
                        resources: ['horizontalpodautoscalers'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['authentication.k8s.io'],
                        resources: ['tokenreviews'],
                        verbs: ['create'],
                    },
                    {
                        apiGroups: ['authorization.k8s.io'],
                        resources: ['subjectaccessreviews'],
                        verbs: ['create'],
                    },
                    {
                        apiGroups: ['policy'],
                        resources: ['poddisruptionbudgets'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['certificates.k8s.io'],
                        resources: ['certificatesigningrequests'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['discovery.k8s.io'],
                        resources: ['endpointslices'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['storage.k8s.io'],
                        resources: ['storageclasses', 'volumeattachments'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['admissionregistration.k8s.io'],
                        resources: [
                            'mutatingwebhookconfigurations',
                            'validatingwebhookconfigurations',
                        ],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['networking.k8s.io'],
                        resources: ['networkpolicies', 'ingressclasses', 'ingresses'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['coordination.k8s.io'],
                        resources: ['leases'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['rbac.authorization.k8s.io'],
                        resources: ['clusterrolebindings', 'clusterroles', 'rolebindings', 'roles'],
                        verbs: ['list', 'watch'],
                    },
                ],
            },
            localOpts,
        )

        // ClusterRoleBinding
        this.clusterRoleBinding = new k8s.rbac.v1.ClusterRoleBinding(
            `${name}-cluster-role-binding`,
            {
                metadata: {
                    name: 'kube-state-metrics',
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
                roleRef: {
                    apiGroup: 'rbac.authorization.k8s.io',
                    kind: 'ClusterRole',
                    name: this.clusterRole.metadata.name,
                },
                subjects: [
                    {
                        kind: 'ServiceAccount',
                        name: this.serviceAccount.metadata.name,
                        namespace: args.namespace,
                    },
                ],
            },
            localOpts,
        )

        // Deployment
        this.deployment = new k8s.apps.v1.Deployment(
            name,
            {
                metadata: {
                    name: 'kube-state-metrics',
                    namespace: args.namespace,
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
                spec: {
                    replicas: 1,
                    selector: {
                        matchLabels: {
                            app: 'kube-state-metrics',
                        },
                    },
                    template: {
                        metadata: {
                            labels: {
                                app: 'kube-state-metrics',
                                'app.kubernetes.io/name': 'kube-state-metrics',
                                'app.kubernetes.io/version': versions.kubeStateMetrics,
                            },
                        },
                        spec: {
                            automountServiceAccountToken: false,
                            securityContext: {
                                fsGroup: 65534,
                                runAsGroup: 65534,
                                runAsNonRoot: true,
                                runAsUser: 65534,
                            },
                            serviceAccountName: this.serviceAccount.metadata.name,
                            containers: [
                                {
                                    name: 'kube-state-metrics',
                                    image: `registry.k8s.io/kube-state-metrics/kube-state-metrics:v${versions.kubeStateMetrics}`,
                                    ports: [
                                        {
                                            containerPort: 8080,
                                            name: 'http-metrics',
                                        },
                                        {
                                            containerPort: 8081,
                                            name: 'telemetry',
                                        },
                                    ],
                                    livenessProbe: {
                                        httpGet: {
                                            path: '/healthz',
                                            port: 8080,
                                        },
                                        initialDelaySeconds: 5,
                                        timeoutSeconds: 5,
                                    },
                                    readinessProbe: {
                                        httpGet: {
                                            path: '/',
                                            port: 8081,
                                        },
                                        initialDelaySeconds: 5,
                                        timeoutSeconds: 5,
                                    },
                                    resources: {
                                        requests: {
                                            cpu: '10m',
                                            memory: '32Mi',
                                        },
                                        limits: {
                                            cpu: '250m',
                                            memory: '256Mi',
                                        },
                                    },
                                    securityContext: {
                                        allowPrivilegeEscalation: false,
                                        capabilities: {
                                            drop: ['ALL'],
                                        },
                                        readOnlyRootFilesystem: true,
                                        runAsNonRoot: true,
                                        runAsUser: 65534,
                                        seccompProfile: {
                                            type: 'RuntimeDefault',
                                        },
                                    },
                                    volumeMounts: [
                                        {
                                            mountPath:
                                                '/var/run/secrets/kubernetes.io/serviceaccount',
                                            name: 'kube-api-access',
                                            readOnly: true,
                                        },
                                    ],
                                },
                            ],
                            nodeSelector: {
                                'kubernetes.io/os': 'linux',
                            },
                            volumes: [
                                {
                                    name: 'kube-api-access',
                                    projected: {
                                        defaultMode: 420,
                                        sources: [
                                            {
                                                serviceAccountToken: {
                                                    expirationSeconds: 3607,
                                                    path: 'token',
                                                },
                                            },
                                            {
                                                configMap: {
                                                    items: [
                                                        {
                                                            key: 'ca.crt',
                                                            path: 'ca.crt',
                                                        },
                                                    ],
                                                    name: 'kube-root-ca.crt',
                                                },
                                            },
                                            {
                                                downwardAPI: {
                                                    items: [
                                                        {
                                                            fieldRef: {
                                                                apiVersion: 'v1',
                                                                fieldPath: 'metadata.namespace',
                                                            },
                                                            path: 'namespace',
                                                        },
                                                    ],
                                                },
                                            },
                                        ],
                                    },
                                },
                            ],
                        },
                    },
                },
            },
            localOpts,
        )

        // Service
        this.service = new k8s.core.v1.Service(
            `${name}-service`,
            {
                metadata: {
                    name: 'kube-state-metrics',
                    namespace: args.namespace,
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
                spec: {
                    type: 'ClusterIP',
                    ports: [
                        {
                            name: 'http-metrics',
                            port: 8080,
                            protocol: 'TCP',
                            targetPort: 'http-metrics',
                        },
                        {
                            name: 'telemetry',
                            port: 8081,
                            protocol: 'TCP',
                            targetPort: 'telemetry',
                        },
                    ],
                    selector: {
                        app: 'kube-state-metrics',
                    },
                },
            },
            localOpts,
        )

        this.registerOutputs({
            deployment: this.deployment,
            service: this.service,
            serviceAccount: this.serviceAccount,
            clusterRole: this.clusterRole,
            clusterRoleBinding: this.clusterRoleBinding,
        })
    }

    public createServiceMonitor(): ServiceMonitor {
        return new ServiceMonitor(
            'kube-state-metrics-sm',
            {
                metadata: {
                    name: 'kube-state-metrics',
                    namespace: this.service.metadata.namespace,
                    labels: {
                        app: 'kube-state-metrics',
                        'app.kubernetes.io/name': 'kube-state-metrics',
                        'app.kubernetes.io/version': versions.kubeStateMetrics,
                    },
                },
                spec: {
                    selector: {
                        matchLabels: {
                            app: 'kube-state-metrics',
                        },
                    },
                    endpoints: [
                        {
                            port: 'http-metrics',
                            interval: '30s',
                            scrapeTimeout: '30s',
                        },
                        {
                            port: 'telemetry',
                            interval: '30s',
                            scrapeTimeout: '30s',
                        },
                    ],
                },
            },
            { parent: this },
        )
    }
}

export interface KubeStateMetricsArgs {
    namespace: pulumi.Input<string>
}
