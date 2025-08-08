import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { ServiceMonitor } from '../prometheus-operator/crds/monitoring/v1'
import { versions } from '../../../.versions'

export class NodeExporter extends pulumi.ComponentResource {
    private args: NodeExporterArgs
    private opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }

    constructor(
        name: string,
        args: NodeExporterArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:node-exporter', name, {}, opts)
        this.args = args
        this.opts = opts
        const localOpts = { ...opts, parent: this }

        // ServiceAccount
        const serviceAccount = new k8s.core.v1.ServiceAccount(
            'node-exporter-sa',
            {
                metadata: {
                    name: 'node-exporter',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
                    },
                },
                automountServiceAccountToken: false,
            },
            localOpts,
        )

        // ClusterRole
        const clusterRole = new k8s.rbac.v1.ClusterRole(
            'node-exporter-cr',
            {
                metadata: {
                    name: 'node-exporter',
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
                    },
                },
                rules: [
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

        // ClusterRoleBinding
        const clusterRoleBinding = new k8s.rbac.v1.ClusterRoleBinding(
            'node-exporter-crb',
            {
                metadata: {
                    name: 'node-exporter',
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
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

        // DaemonSet
        const daemonSet = new k8s.apps.v1.DaemonSet(
            'node-exporter-ds',
            {
                metadata: {
                    name: 'node-exporter',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
                    },
                },
                spec: {
                    selector: {
                        matchLabels: {
                            'app.kubernetes.io/component': 'exporter',
                            'app.kubernetes.io/name': 'node-exporter',
                            'app.kubernetes.io/part-of': 'kube-prometheus',
                        },
                    },
                    template: {
                        metadata: {
                            annotations: {
                                'kubectl.kubernetes.io/default-container': 'node-exporter',
                            },
                            labels: {
                                'app.kubernetes.io/component': 'exporter',
                                'app.kubernetes.io/name': 'node-exporter',
                                'app.kubernetes.io/part-of': 'kube-prometheus',
                                'app.kubernetes.io/version': versions.nodeExporter,
                            },
                        },
                        spec: {
                            automountServiceAccountToken: true,
                            serviceAccountName: serviceAccount.metadata.name,
                            securityContext: {
                                runAsGroup: 65534,
                                runAsNonRoot: true,
                                runAsUser: 65534,
                            },
                            hostNetwork: true,
                            hostPID: true,
                            priorityClassName: 'system-cluster-critical',
                            tolerations: [
                                {
                                    operator: 'Exists',
                                },
                            ],
                            containers: [
                                {
                                    name: 'node-exporter',
                                    image: `quay.io/prometheus/node-exporter:v${versions.nodeExporter}`,
                                    args: [
                                        '--web.listen-address=127.0.0.1:9101',
                                        '--path.sysfs=/host/sys',
                                        '--path.rootfs=/host/root',
                                        '--path.procfs=/host/root/proc',
                                        '--path.udev.data=/host/root/run/udev/data',
                                        '--no-collector.wifi',
                                        '--no-collector.hwmon',
                                        '--no-collector.btrfs',
                                        '--collector.filesystem.mount-points-exclude=^/(dev|proc|sys|run/k3s/containerd/.+|var/lib/docker/.+|var/lib/kubelet/pods/.+)($|/)',
                                        '--collector.netclass.ignored-devices=^(veth.*|[a-f0-9]{15})$',
                                        '--collector.netdev.device-exclude=^(veth.*|[a-f0-9]{15})$',
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
                                            add: ['SYS_TIME'],
                                            drop: ['ALL'],
                                        },
                                        readOnlyRootFilesystem: true,
                                    },
                                    volumeMounts: [
                                        {
                                            mountPath: '/host/sys',
                                            mountPropagation: 'HostToContainer',
                                            name: 'sys',
                                            readOnly: true,
                                        },
                                        {
                                            mountPath: '/host/root',
                                            mountPropagation: 'HostToContainer',
                                            name: 'root',
                                            readOnly: true,
                                        },
                                    ],
                                },
                                {
                                    name: 'kube-rbac-proxy',
                                    image: 'quay.io/brancz/kube-rbac-proxy:v0.19.1',
                                    args: [
                                        '--secure-listen-address=[$(IP)]:9100',
                                        '--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305',
                                        '--upstream=http://127.0.0.1:9101/',
                                    ],
                                    env: [
                                        {
                                            name: 'IP',
                                            valueFrom: {
                                                fieldRef: {
                                                    fieldPath: 'status.podIP',
                                                },
                                            },
                                        },
                                    ],
                                    ports: [
                                        {
                                            containerPort: 9100,
                                            hostPort: 9100,
                                            name: 'https',
                                            protocol: 'TCP',
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
                            nodeSelector: {
                                'kubernetes.io/os': 'linux',
                            },
                            volumes: [
                                {
                                    name: 'sys',
                                    hostPath: {
                                        path: '/sys',
                                    },
                                },
                                {
                                    name: 'root',
                                    hostPath: {
                                        path: '/',
                                    },
                                },
                            ],
                        },
                    },
                    updateStrategy: {
                        type: 'RollingUpdate',
                        rollingUpdate: {
                            maxUnavailable: '10%',
                        },
                    },
                },
            },
            { ...localOpts, dependsOn: [serviceAccount, clusterRole, clusterRoleBinding] },
        )

        // Service
        const service = new k8s.core.v1.Service(
            'node-exporter-svc',
            {
                metadata: {
                    name: 'node-exporter',
                    namespace: args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
                    },
                },
                spec: {
                    clusterIP: 'None',
                    ports: [
                        {
                            name: 'https',
                            port: 9100,
                            targetPort: 'https',
                        },
                    ],
                    selector: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                    },
                },
            },
            localOpts,
        )

        this.registerOutputs({
            serviceAccount,
            clusterRole,
            clusterRoleBinding,
            daemonSet,
            service,
        })
    }

    createServiceMonitor(): ServiceMonitor {
        const localOpts = { ...this.opts, parent: this }

        return new ServiceMonitor(
            'node-exporter-sm',
            {
                metadata: {
                    name: 'node-exporter',
                    namespace: this.args.namespace,
                    labels: {
                        'app.kubernetes.io/component': 'exporter',
                        'app.kubernetes.io/name': 'node-exporter',
                        'app.kubernetes.io/part-of': 'kube-prometheus',
                        'app.kubernetes.io/version': versions.nodeExporter,
                    },
                },
                spec: {
                    endpoints: [
                        {
                            bearerTokenFile: '/var/run/secrets/kubernetes.io/serviceaccount/token',
                            interval: '15s',
                            port: 'https',
                            relabelings: [
                                {
                                    action: 'replace',
                                    regex: '(.*)',
                                    replacement: '$1',
                                    sourceLabels: ['__meta_kubernetes_pod_node_name'],
                                    targetLabel: 'instance',
                                },
                            ],
                            scheme: 'https',
                            tlsConfig: {
                                insecureSkipVerify: true,
                            },
                        },
                    ],
                    jobLabel: 'app.kubernetes.io/name',
                    selector: {
                        matchLabels: {
                            'app.kubernetes.io/component': 'exporter',
                            'app.kubernetes.io/name': 'node-exporter',
                            'app.kubernetes.io/part-of': 'kube-prometheus',
                        },
                    },
                },
            },
            localOpts,
        )
    }
}

export interface NodeExporterArgs {
    namespace: pulumi.Input<string>
}