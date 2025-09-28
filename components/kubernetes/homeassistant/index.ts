import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as fs from 'fs'
import * as path from 'path'
import { Gateway } from '../gateway'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'

export class HomeAssistant extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: HomeAssistantArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:homeassistant', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // Load configuration files from extracted config or use defaults
        const configData = this.loadConfigurationFiles(args)

        // ConfigMap for Home Assistant configuration
        const configMap = new k8s.core.v1.ConfigMap(
            `${name}-config`,
            {
                metadata: {
                    name: 'homeassistant-config',
                    namespace: args.namespace,
                },
                data: configData,
            },
            localOpts,
        )

        const statefulSet = new k8s.apps.v1.StatefulSet(
            name,
            {
                metadata: {
                    name: 'homeassistant',
                    namespace: args.namespace,
                    labels: { app: 'homeassistant' },
                },
                spec: {
                    serviceName: 'homeassistant-web',
                    replicas: 1,
                    selector: {
                        matchLabels: { app: 'homeassistant' },
                    },
                    template: {
                        metadata: {
                            labels: { app: 'homeassistant' },
                        },
                        spec: {
                            securityContext: {
                                fsGroup: 1000,
                                runAsUser: 1000,
                                runAsGroup: 1000,
                                runAsNonRoot: true,
                            },
                            initContainers: [
                                {
                                    name: 'config-init',
                                    image:
                                        args.image ||
                                        `ghcr.io/home-assistant/home-assistant:${args.version}`,
                                    command: ['/bin/sh'],
                                    args: [
                                        '-c',
                                        `echo "Copying configuration files..."
                                        cp /tmp/config/*.yaml /config/ 2>/dev/null || true
                                        cp /tmp/config/*.yml /config/ 2>/dev/null || true
                                        
                                        # Install custom components from image staging area
                                        if [ -d /usr/src/homeassistant/custom_components_staging ]; then
                                            echo "Installing custom components from image..."
                                            mkdir -p /config/custom_components
                                            cp -r /usr/src/homeassistant/custom_components_staging/* /config/custom_components/
                                            echo "Custom components installed:"
                                            ls -la /config/custom_components/
                                        fi
                                        
                                        # Install custom components from config if present
                                        if [ -d /tmp/config/custom_components ]; then
                                            echo "Installing custom components from config..."
                                            mkdir -p /config/custom_components
                                            cp -r /tmp/config/custom_components/* /config/custom_components/
                                        fi
                                        
                                        echo "Configuration files copied successfully"
                                        ls -la /config/*.y*ml 2>/dev/null || echo "No YAML files found"`,
                                    ],
                                    volumeMounts: [
                                        {
                                            name: 'data',
                                            mountPath: '/config',
                                        },
                                        {
                                            name: 'config-files',
                                            mountPath: '/tmp/config',
                                            readOnly: true,
                                        },
                                    ],
                                    securityContext: {
                                        runAsUser: 1000,
                                        runAsGroup: 1000,
                                        runAsNonRoot: true,
                                    },
                                },
                            ],
                            containers: [
                                {
                                    name: 'homeassistant',
                                    image:
                                        args.image ||
                                        `ghcr.io/home-assistant/home-assistant:${args.version}`,
                                    ports: [
                                        {
                                            containerPort: 8123,
                                            name: 'web',
                                            protocol: 'TCP',
                                        },
                                    ],
                                    env: [
                                        {
                                            name: 'TZ',
                                            value: 'UTC',
                                        },
                                    ],
                                    livenessProbe: {
                                        httpGet: {
                                            path: '/',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 60,
                                        timeoutSeconds: 10,
                                        periodSeconds: 60,
                                    },
                                    readinessProbe: {
                                        httpGet: {
                                            path: '/',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 30,
                                        timeoutSeconds: 10,
                                        periodSeconds: 10,
                                    },
                                    resources: {
                                        requests: {
                                            cpu: '100m',
                                            memory: '256Mi',
                                        },
                                        limits: {
                                            cpu: '1000m',
                                            memory: '1Gi',
                                        },
                                    },
                                    volumeMounts: [
                                        {
                                            name: 'data',
                                            mountPath: '/config',
                                        },
                                    ],
                                },
                            ],
                            volumes: [
                                {
                                    name: 'config-files',
                                    configMap: {
                                        name: configMap.metadata.name,
                                    },
                                },
                            ],
                        },
                    },
                    volumeClaimTemplates: [
                        {
                            metadata: {
                                name: 'data',
                            },
                            spec: {
                                accessModes: ['ReadWriteOnce'],
                                storageClassName: args.storage.storageClassName,
                                resources: {
                                    requests: {
                                        storage: args.storage.size,
                                    },
                                },
                            },
                        },
                    ],
                },
            },
            localOpts,
        )

        const service = new k8s.core.v1.Service(
            `${name}-service`,
            {
                metadata: {
                    name: 'homeassistant-web',
                    namespace: args.namespace,
                    labels: statefulSet.metadata.labels,
                },
                spec: {
                    type: 'ClusterIP',
                    ports: [
                        {
                            name: 'web',
                            port: 8123,
                            protocol: 'TCP',
                            targetPort: 'web',
                        },
                    ],
                    selector: statefulSet.spec.template.metadata.labels,
                },
            },
            localOpts,
        )

        const gateway = new Gateway(
            name,
            {
                namespace: args.namespace,
                hostname: args.web.hostname,
                serviceName: service.metadata.name,
                servicePort: 8123,
                issuer: args.web.issuer,
                tailscale: args.web.tailscale,
            },
            localOpts,
        )

        this.registerOutputs({
            configMap,
            statefulSet,
            service,
            gateway,
        })
    }

    private loadConfigurationFiles(args: HomeAssistantArgs): Record<string, string> {
        const configDir = path.join(__dirname, 'config')

        // Check if config directory exists
        if (!fs.existsSync(configDir)) {
            throw new Error(
                `Configuration directory not found: ${configDir}\n` +
                    'Run "yarn homeassistant" first to extract configuration from the running pod.',
            )
        }

        const configData: Record<string, string> = {}

        // Load all YAML files from the config directory
        const configFiles = fs.readdirSync(configDir)
        for (const file of configFiles) {
            const filePath = path.join(configDir, file)
            const stat = fs.statSync(filePath)

            if (stat.isFile() && (file.endsWith('.yaml') || file.endsWith('.yml'))) {
                const content = fs.readFileSync(filePath, 'utf8')
                configData[file] = content
                console.log(`Loaded Home Assistant config: ${file}`)
            } else if (stat.isDirectory() && file === 'custom_components') {
                // Handle custom_components directory - it will be copied by init container
                console.log(`Found custom_components directory`)
            }
        }

        // Ensure we have at least configuration.yaml
        if (!configData['configuration.yaml']) {
            throw new Error(
                'configuration.yaml not found in config directory.\n' +
                    'Run "yarn homeassistant" first to extract configuration from the running pod.',
            )
        }

        return configData
    }
}

export interface HomeAssistantArgs {
    namespace: pulumi.Input<string>
    version: string
    image?: string // Allow custom image override
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
    configuration?: string
}
