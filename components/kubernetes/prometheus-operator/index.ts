import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as path from 'path'
import { versions } from '../../../.versions'

export class PrometheusOperator extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: PrometheusOperatorArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:prometheus-operator', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // Install CRDs from the downloaded crds.yaml
        const crdsPath = path.join(__dirname, 'crds.yaml')
        const crds = new k8s.yaml.ConfigFile(
            'prometheus-operator-crds',
            {
                file: crdsPath,
            },
            localOpts,
        )

        const serviceAccount = new k8s.core.v1.ServiceAccount(
            'prometheus-operator-sa',
            {
                metadata: {
                    name: 'prometheus-operator',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheusOperator,
                    },
                },
                automountServiceAccountToken: false,
            },
            localOpts,
        )

        const clusterRole = new k8s.rbac.v1.ClusterRole(
            'prometheus-operator-cr',
            {
                metadata: {
                    name: 'prometheus-operator',
                    labels: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheusOperator,
                    },
                },
                rules: [
                    {
                        apiGroups: ['monitoring.coreos.com'],
                        resources: [
                            'alertmanagers',
                            'alertmanagers/finalizers',
                            'alertmanagers/status',
                            'alertmanagerconfigs',
                            'prometheuses',
                            'prometheuses/finalizers',
                            'prometheuses/status',
                            'prometheusagents',
                            'prometheusagents/finalizers',
                            'prometheusagents/status',
                            'thanosrulers',
                            'thanosrulers/finalizers',
                            'thanosrulers/status',
                            'scrapeconfigs',
                            'servicemonitors',
                            'servicemonitors/status',
                            'podmonitors',
                            'probes',
                            'prometheusrules',
                        ],
                        verbs: ['*'],
                    },
                    {
                        apiGroups: ['apps'],
                        resources: ['statefulsets'],
                        verbs: ['*'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['configmaps', 'secrets'],
                        verbs: ['*'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['pods'],
                        verbs: ['list', 'delete'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['services', 'services/finalizers'],
                        verbs: ['get', 'create', 'update', 'delete'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['nodes'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['namespaces'],
                        verbs: ['get', 'list', 'watch'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['events'],
                        verbs: ['patch', 'create'],
                    },
                    {
                        apiGroups: ['networking.k8s.io'],
                        resources: ['ingresses'],
                        verbs: ['get', 'list', 'watch'],
                    },
                    {
                        apiGroups: ['storage.k8s.io'],
                        resources: ['storageclasses'],
                        verbs: ['get'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['endpoints'],
                        verbs: ['get', 'create', 'update', 'delete'],
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
                ],
            },
            localOpts,
        )

        const clusterRoleBinding = new k8s.rbac.v1.ClusterRoleBinding(
            'prometheus-operator-crb',
            {
                metadata: {
                    name: 'prometheus-operator',
                    labels: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheusOperator,
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

        const service = new k8s.core.v1.Service(
            'prometheus-operator-svc',
            {
                metadata: {
                    name: 'prometheus-operator',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheusOperator,
                    },
                },
                spec: {
                    clusterIP: 'None',
                    ports: [
                        {
                            name: 'https',
                            port: 8443,
                            targetPort: 'https',
                        },
                    ],
                    selector: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                    },
                },
            },
            localOpts,
        )

        const deployment = new k8s.apps.v1.Deployment(
            'prometheus-operator-deployment',
            {
                metadata: {
                    name: 'prometheus-operator',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'controller',
                        'app.kubernetes.io/name': 'prometheus-operator',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.prometheusOperator,
                    },
                },
                spec: {
                    replicas: 1,
                    selector: {
                        matchLabels: {
                            'app.kubernetes.io/component': 'controller',
                            'app.kubernetes.io/name': 'prometheus-operator',
                            'app.kubernetes.io/part-of': 'kube-prometheus',
                        },
                    },
                    template: {
                        metadata: {
                            annotations: {
                                'kubectl.kubernetes.io/default-container': 'prometheus-operator',
                            },
                            labels: {
                                'app.kubernetes.io/component': 'controller',
                                'app.kubernetes.io/name': 'prometheus-operator',
                                'app.kubernetes.io/part-of': 'kube-prometheus',
                                'app.kubernetes.io/version': versions.prometheusOperator,
                            },
                        },
                        spec: {
                            automountServiceAccountToken: true,
                            serviceAccountName: serviceAccount.metadata.name,
                            securityContext: {
                                runAsGroup: 65534,
                                runAsNonRoot: true,
                                runAsUser: 65534,
                                seccompProfile: {
                                    type: 'RuntimeDefault',
                                },
                            },
                            nodeSelector: {
                                'kubernetes.io/os': 'linux',
                            },
                            containers: [
                                {
                                    name: 'prometheus-operator',
                                    image: `quay.io/prometheus-operator/prometheus-operator:v${versions.prometheusOperator}`,
                                    args: [
                                        '--kubelet-service=kube-system/kubelet',
                                        `--prometheus-config-reloader=quay.io/prometheus-operator/prometheus-config-reloader:v${versions.prometheusOperator}`,
                                        '--kubelet-endpoints=true',
                                        '--kubelet-endpointslice=false',
                                    ],
                                    env: [
                                        {
                                            name: 'GOGC',
                                            value: '30',
                                        },
                                    ],
                                    ports: [
                                        {
                                            containerPort: 8080,
                                            name: 'http',
                                        },
                                    ],
                                    resources: {
                                        limits: {
                                            cpu: '100m',
                                            memory: '64Mi',
                                        },
                                        requests: {
                                            cpu: '10m',
                                            memory: '32Mi',
                                        },
                                    },
                                    securityContext: {
                                        allowPrivilegeEscalation: false,
                                        capabilities: {
                                            drop: ['ALL'],
                                        },
                                        readOnlyRootFilesystem: true,
                                    },
                                },
                                {
                                    name: 'kube-rbac-proxy',
                                    image: 'quay.io/brancz/kube-rbac-proxy:v0.19.1',
                                    args: [
                                        '--secure-listen-address=:8443',
                                        '--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305',
                                        '--upstream=http://127.0.0.1:8080/',
                                    ],
                                    ports: [
                                        {
                                            containerPort: 8443,
                                            name: 'https',
                                        },
                                    ],
                                    resources: {
                                        limits: {
                                            cpu: '20m',
                                            memory: '40Mi',
                                        },
                                        requests: {
                                            cpu: '10m',
                                            memory: '20Mi',
                                        },
                                    },
                                    securityContext: {
                                        allowPrivilegeEscalation: false,
                                        capabilities: {
                                            drop: ['ALL'],
                                        },
                                        readOnlyRootFilesystem: true,
                                        runAsGroup: 65532,
                                        runAsNonRoot: true,
                                        runAsUser: 65532,
                                        seccompProfile: {
                                            type: 'RuntimeDefault',
                                        },
                                    },
                                },
                            ],
                        },
                    },
                },
            },
            { ...localOpts, dependsOn: [serviceAccount, clusterRole, clusterRoleBinding, crds] },
        )

        this.registerOutputs({
            crds,
            serviceAccount,
            clusterRole,
            clusterRoleBinding,
            service,
            deployment,
        })
    }
}

export interface PrometheusOperatorArgs {
    namespace: pulumi.Input<string>
}
