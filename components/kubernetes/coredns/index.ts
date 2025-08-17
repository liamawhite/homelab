import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as fs from 'fs'
import * as path from 'path'
import { versions } from '../../../.versions'

function loadBlocklists(): string {
    const blocklistDir = path.join(__dirname, 'blocklists')
    let combinedBlocklist = '# Combined blocklists\n'

    try {
        const files = fs.readdirSync(blocklistDir)
        for (const file of files) {
            if (file.endsWith('.txt')) {
                const filePath = path.join(blocklistDir, file)
                const content = fs.readFileSync(filePath, 'utf8')
                combinedBlocklist += `\n# From ${file}\n${content}\n`
            }
        }
    } catch (error) {
        console.warn('Could not load blocklists:', error)
        combinedBlocklist += '# No blocklists found\n'
    }

    return combinedBlocklist
}

export class CoreDns extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: CoreDnsArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:coredns', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const serviceAccount = new k8s.core.v1.ServiceAccount(
            `${name}-sa`,
            {
                metadata: {
                    name: 'coredns-external',
                    namespace: args.namespace,
                },
            },
            localOpts,
        )

        const clusterRole = new k8s.rbac.v1.ClusterRole(
            `${name}-cr`,
            {
                metadata: {
                    name: 'coredns-external',
                },
                rules: [
                    {
                        apiGroups: ['apiextensions.k8s.io'],
                        resources: ['customresourcedefinitions'],
                        verbs: ['get', 'list', 'watch'],
                    },
                    {
                        apiGroups: ['extensions', 'networking.k8s.io'],
                        resources: ['ingresses'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: [''],
                        resources: ['services', 'namespaces'],
                        verbs: ['list', 'watch'],
                    },
                    {
                        apiGroups: ['gateway.networking.k8s.io'],
                        resources: ['*'],
                        verbs: ['watch', 'list'],
                    },
                    {
                        apiGroups: ['externaldns.k8s.io'],
                        resources: ['dnsendpoints'],
                        verbs: ['get', 'watch', 'list'],
                    },
                    {
                        apiGroups: ['externaldns.k8s.io'],
                        resources: ['dnsendpoints/status'],
                        verbs: ['*'],
                    },
                ],
            },
            localOpts,
        )

        const clusterRoleBinding = new k8s.rbac.v1.ClusterRoleBinding(
            `${name}-crb`,
            {
                metadata: {
                    name: 'coredns-external',
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

        const configMap = new k8s.core.v1.ConfigMap(
            name,
            {
                metadata: { namespace: args.namespace },
                data: {
                    Corefile: pulumi.interpolate`
.:53 {
    log
    errors
    health :8080
    ready :8181
    prometheus :9153
    
    # Block ads and malware
    hosts /etc/coredns/blocklist {
        fallthrough
    }

    # Use http routes to resolve .homelab domains
    k8s_gateway homelab {
        resources HTTPRoute
    }

    # Forward all other queries to Cloudflare DNS
    forward . 1.1.1.1
    
    cache 30
    loop
    reload
    loadbalance
}`,
                    blocklist: loadBlocklists(),
                },
            },
            localOpts,
        )

        const deploy = new k8s.apps.v1.Deployment(
            name,
            {
                metadata: {
                    name: 'coredns-external',
                    namespace: args.namespace,
                    labels: { app: 'coredns-external' },
                },
                spec: {
                    replicas: 2,
                    selector: {
                        matchLabels: { app: 'coredns-external' },
                    },
                    strategy: {
                        type: 'RollingUpdate',
                        rollingUpdate: { maxSurge: 1, maxUnavailable: 1 },
                    },
                    template: {
                        metadata: { labels: { app: 'coredns-external' } },
                        spec: {
                            serviceAccountName: serviceAccount.metadata.name,
                            containers: [
                                {
                                    name: 'coredns',
                                    image: `ghcr.io/liamawhite/coredns:${versions.coredns}`,
                                    args: ['-conf', '/etc/coredns/Corefile'],
                                    ports: [
                                        {
                                            containerPort: 53,
                                            name: 'dns',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 53,
                                            name: 'dns-udp',
                                            protocol: 'UDP',
                                        },
                                        {
                                            containerPort: 8080,
                                            name: 'health',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 8181,
                                            name: 'ready',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 9153,
                                            name: 'metrics',
                                            protocol: 'TCP',
                                        },
                                    ],
                                    livenessProbe: {
                                        httpGet: {
                                            path: '/health',
                                            port: 'health',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 10,
                                        timeoutSeconds: 5,
                                    },
                                    readinessProbe: {
                                        httpGet: {
                                            path: '/ready',
                                            port: 'ready',
                                            scheme: 'HTTP',
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
                                            cpu: '100m',
                                            memory: '64Mi',
                                        },
                                    },
                                    volumeMounts: [
                                        {
                                            name: 'config',
                                            mountPath: '/etc/coredns',
                                            readOnly: true,
                                        },
                                    ],
                                },
                            ],
                            volumes: [
                                {
                                    name: 'config',
                                    configMap: {
                                        name: configMap.metadata.name,
                                    },
                                },
                            ],
                        },
                    },
                },
            },
            localOpts,
        )

        const service = new k8s.core.v1.Service(
            name,
            {
                metadata: {
                    name: 'coredns-external',
                    namespace: args.namespace,
                    labels: deploy.metadata.labels,
                    annotations: args.dns?.annotations,
                },
                spec: {
                    type: 'LoadBalancer',
                    ports: [
                        {
                            name: 'dns-tcp',
                            port: 53,
                            protocol: 'TCP',
                            targetPort: 'dns',
                        },
                        {
                            name: 'dns-udp',
                            port: 53,
                            protocol: 'UDP',
                            targetPort: 'dns-udp',
                        },
                        {
                            name: 'metrics',
                            port: 9153,
                            protocol: 'TCP',
                            targetPort: 'metrics',
                        },
                    ],
                    selector: deploy.spec.template.metadata.labels,
                },
            },
            localOpts,
        )

        this.registerOutputs({
            serviceAccount,
            clusterRole,
            clusterRoleBinding,
            configMap,
            deploy,
            service,
        })
    }
}

export interface CoreDnsArgs {
    namespace: pulumi.Input<string>
    dns?: {
        annotations?: {
            [key: string]: pulumi.Input<string>
        }
    }
    homelabTLDForwarder: pulumi.Input<string>
}
