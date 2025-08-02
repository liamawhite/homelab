import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as fs from 'fs'
import * as path from 'path'
import * as labels from '../istio/labels'
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
    readonly namespace: pulumi.Output<string>
    readonly localAddress: pulumi.Output<string>

    constructor(
        name: string,
        args: CoreDnsArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:coredns', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const ns = new k8s.core.v1.Namespace(
            name,
            {
                metadata: {
                    name: 'coredns-external',
                    labels: labels.namespace.enableAmbient,
                },
            },
            localOpts,
        )
        this.namespace = ns.metadata.name

        const configMap = new k8s.core.v1.ConfigMap(
            name,
            {
                metadata: { namespace: ns.metadata.name },
                data: {
                    Corefile: `.:53 {
    errors
    health :8080
    ready :8181
    prometheus :9153
    
    # Block ads and malware
    hosts /etc/coredns/blocklist {
        fallthrough
    }
    
    # Forward to upstream resolvers
    forward . 1.1.1.1 8.8.8.8 {
        policy round_robin
        health_check 5s
    }
    
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
                    namespace: this.namespace,
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
                            containers: [
                                {
                                    name: 'coredns',
                                    image: `coredns/coredns:${versions.coredns}`,
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
                                            cpu: '100m',
                                            memory: '128Mi',
                                        },
                                        limits: {
                                            cpu: '500m',
                                            memory: '256Mi',
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
                    namespace: this.namespace,
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

        this.localAddress = pulumi.interpolate`${service.metadata.name}.${this.namespace}.svc.cluster.local:53`

        this.registerOutputs({
            deploy,
            service,
            configMap,
        })
    }
}

export interface CoreDnsArgs {
    dns?: {
        annotations?: {
            [key: string]: pulumi.Input<string>
        }
    }
}
