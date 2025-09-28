import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as fs from 'fs'
import * as path from 'path'
import { Gateway } from '../gateway'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'

export interface SyncthingDevice {
    id: string
    name: string
}

export interface SyncthingFolder {
    id: string
    path: string
    devices: string[]
    type?: 'sendreceive' | 'sendonly' | 'receiveonly'
}

export interface SyncthingDeclarativeConfig {
    devices: Record<string, SyncthingDevice>
    folders: Record<string, SyncthingFolder>
}

function generateSyncthingConfig(config: SyncthingDeclarativeConfig): string {
    const templatePath = path.join(__dirname, 'config-template.xml')
    const template = fs.readFileSync(templatePath, 'utf8')

    const devicesXml = Object.entries(config.devices)
        .map(
            ([key, device]) =>
                `    <device id="${device.id}" name="${device.name}" compression="metadata" introducer="false" skipIntroductionRemovals="false" introducedBy="">
        <address>dynamic</address>
        <paused>false</paused>
        <autoAcceptFolders>false</autoAcceptFolders>
        <maxSendKbps>0</maxSendKbps>
        <maxRecvKbps>0</maxRecvKbps>
        <maxRequestKiB>0</maxRequestKiB>
        <untrusted>false</untrusted>
        <remoteGUIPort>0</remoteGUIPort>
    </device>`,
        )
        .join('\n')

    const foldersXml = Object.entries(config.folders)
        .map(([key, folder]) => {
            const devicesInFolder = folder.devices
                .map(deviceKey => config.devices[deviceKey]?.id)
                .filter(Boolean)
                .map(deviceId => `        <device id="${deviceId}" introducedBy=""></device>`)
                .join('\n')

            return `    <folder id="${folder.id}" label="${key}" path="${folder.path}" type="${folder.type || 'sendreceive'}" rescanIntervalS="3600" fsWatcherEnabled="true" fsWatcherDelayS="10" ignorePerms="false" autoNormalize="true">
${devicesInFolder}
        <minDiskFree unit="%">1</minDiskFree>
        <versioning>
            <param key="cleanupIntervalS" val="3600"></param>
            <param key="command" val=""></param>
            <param key="maxAge" val="365"></param>
        </versioning>
        <copiers>0</copiers>
        <pullerMaxPendingKiB>0</pullerMaxPendingKiB>
        <hashers>0</hashers>
        <order>random</order>
        <ignoreDelete>false</ignoreDelete>
        <scanProgressIntervalS>0</scanProgressIntervalS>
        <pullerPauseS>0</pullerPauseS>
        <maxConflicts>10</maxConflicts>
        <disableSparseFiles>false</disableSparseFiles>
        <disableTempIndexes>false</disableTempIndexes>
        <paused>false</paused>
        <weakHashThresholdPct>25</weakHashThresholdPct>
        <markerName>.stfolder</markerName>
        <copyOwnershipFromParent>false</copyOwnershipFromParent>
        <modTimeWindowS>0</modTimeWindowS>
        <maxConcurrentWrites>2</maxConcurrentWrites>
        <disableFsync>false</disableFsync>
        <blockPullOrder>standard</blockPullOrder>
        <copyRangeMethod>standard</copyRangeMethod>
        <caseSensitiveFS>true</caseSensitiveFS>
        <junctionsAsDirs>false</junctionsAsDirs>
        <syncOwnership>false</syncOwnership>
        <sendOwnership>false</sendOwnership>
        <syncXattrs>false</syncXattrs>
        <sendXattrs>false</sendXattrs>
        <xchacha20poly1305>false</xchacha20poly1305>
    </folder>`
        })
        .join('\n')

    return template.replace('{{FOLDERS}}', foldersXml).replace('{{DEVICES}}', devicesXml)
}

export class Syncthing extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: SyncthingArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:syncthing', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const configMap = new k8s.core.v1.ConfigMap(
            `${name}-config`,
            {
                metadata: {
                    name: 'syncthing-config',
                    namespace: args.namespace,
                },
                data: {
                    'config.xml': generateSyncthingConfig(args.declarativeConfig),
                },
            },
            localOpts,
        )

        const statefulSet = new k8s.apps.v1.StatefulSet(
            name,
            {
                metadata: {
                    name: 'syncthing',
                    namespace: args.namespace,
                    labels: { app: 'syncthing' },
                },
                spec: {
                    serviceName: 'syncthing-web',
                    replicas: 1,
                    selector: {
                        matchLabels: { app: 'syncthing' },
                    },
                    template: {
                        metadata: {
                            labels: { app: 'syncthing' },
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
                                    image: `syncthing/syncthing:${args.version}`,
                                    command: ['/bin/sh'],
                                    args: [
                                        '-c',
                                        `echo "Copying declarative config..."
                                        mkdir -p /var/syncthing/config
                                        cp /tmp/syncthing-config/config.xml /var/syncthing/config/config.xml
                                        echo "Config copied successfully"`,
                                    ],
                                    volumeMounts: [
                                        {
                                            name: 'data',
                                            mountPath: '/var/syncthing',
                                        },
                                        {
                                            name: 'config-template',
                                            mountPath: '/tmp/syncthing-config',
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
                                    name: 'syncthing',
                                    image: `syncthing/syncthing:${args.version}`,
                                    ports: [
                                        {
                                            containerPort: 8384,
                                            name: 'web',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 22000,
                                            name: 'sync-tcp',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 22000,
                                            name: 'sync-udp',
                                            protocol: 'UDP',
                                        },
                                        {
                                            containerPort: 21027,
                                            name: 'discovery',
                                            protocol: 'UDP',
                                        },
                                    ],
                                    env: [
                                        {
                                            name: 'PUID',
                                            value: '1000',
                                        },
                                        {
                                            name: 'PGID',
                                            value: '1000',
                                        },
                                    ],
                                    livenessProbe: {
                                        httpGet: {
                                            path: '/rest/noauth/health',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 30,
                                        timeoutSeconds: 10,
                                        periodSeconds: 60,
                                    },
                                    readinessProbe: {
                                        httpGet: {
                                            path: '/rest/noauth/health',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 10,
                                        timeoutSeconds: 10,
                                        periodSeconds: 10,
                                    },
                                    resources: {
                                        requests: {
                                            cpu: '100m',
                                            memory: '128Mi',
                                        },
                                        limits: {
                                            cpu: '1000m',
                                            memory: '512Mi',
                                        },
                                    },
                                    volumeMounts: [
                                        {
                                            name: 'data',
                                            mountPath: '/var/syncthing',
                                        },
                                    ],
                                },
                            ],
                            volumes: [
                                {
                                    name: 'config-template',
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

        const webService = new k8s.core.v1.Service(
            `${name}-frontend`,
            {
                metadata: {
                    name: 'syncthing-frontend',
                    namespace: args.namespace,
                    labels: statefulSet.metadata.labels,
                },
                spec: {
                    type: 'ClusterIP',
                    ports: [
                        {
                            name: 'web',
                            port: 8384,
                            protocol: 'TCP',
                            targetPort: 'web',
                        },
                    ],
                    selector: statefulSet.spec.template.metadata.labels,
                },
            },
            localOpts,
        )

        const syncService = new k8s.core.v1.Service(
            `${name}-sync`,
            {
                metadata: {
                    name: 'syncthing-sync',
                    namespace: args.namespace,
                    labels: statefulSet.metadata.labels,
                    annotations: args.sync?.annotations,
                },
                spec: {
                    type: 'LoadBalancer',
                    ports: [
                        {
                            name: 'sync-tcp',
                            port: 22000,
                            protocol: 'TCP',
                            targetPort: 'sync-tcp',
                        },
                        {
                            name: 'sync-udp',
                            port: 22000,
                            protocol: 'UDP',
                            targetPort: 'sync-udp',
                        },
                        {
                            name: 'discovery',
                            port: 21027,
                            protocol: 'UDP',
                            targetPort: 'discovery',
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
                serviceName: webService.metadata.name,
                servicePort: 8384,
                issuer: args.web.issuer,
                tailscale: args.web.tailscale,
            },
            localOpts,
        )

        this.registerOutputs({
            configMap,
            statefulSet,
            webService,
            syncService,
            gateway,
        })
    }
}

export interface SyncthingArgs {
    namespace: pulumi.Input<string>
    version: string
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
    sync?: {
        annotations?: {
            [key: string]: pulumi.Input<string>
        }
    }
    declarativeConfig: SyncthingDeclarativeConfig
}
