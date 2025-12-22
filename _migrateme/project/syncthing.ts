import * as k8s from '@pulumi/kubernetes'
import { Syncthing } from '../components/kubernetes/syncthing'
import { configureCluster } from './cluster'
import { configurePki } from './pki'
import { configureStorage } from './storage'
import { versions } from '../.versions'
import * as labels from '../components/kubernetes/istio/labels'

export function configureSyncthing({
    cluster,
    pki,
    storage,
}: {
    cluster: ReturnType<typeof configureCluster>
    pki: ReturnType<typeof configurePki>
    storage: ReturnType<typeof configureStorage>
}) {
    const opts = { provider: cluster.provider, dependsOn: storage.ready }

    const namespace = new k8s.core.v1.Namespace(
        'syncthing-namespace',
        {
            metadata: {
                name: 'syncthing',
                labels: labels.namespace.enableAmbient,
            },
        },
        opts,
    )

    const syncthing = new Syncthing(
        'syncthing',
        {
            namespace: namespace.metadata.name,
            version: versions.syncthing,
            storage: {
                size: '100Gi',
                storageClassName: storage.defaultStorageClass.metadata.name,
            },
            web: {
                hostname: 'syncthing.homelab',
                issuer: pki.issuer.issuerRef(),
                tailscale: {
                    enabled: true,
                    hostname: 'syncthing',
                },
            },
            declarativeConfig: {
                devices: {
                    'kubernetes-syncthing-mirror': {
                        id: 'WWRWTXL-2QLS6J7-QNSA5MZ-XLLUS4R-TKBA754-GXYFAEF-33IO7G4-LH5FIAW',
                        name: 'kubernetes-syncthing',
                    },
                    'macstudio-personal-2023': {
                        id: 'Y6NSC7H-7ISVMIQ-MG6CA4Z-TIXVGBU-CPCRHHN-EAKVOKC-RVWTF45-ZIEANQ7',
                        name: 'macstudio-personal-2023',
                    },
                    'macbookpro-docusign-2025': {
                        id: 'VNBUOSV-NBA7BXM-FCQ2KWS-5YIJNTQ-ZXP2LI6-6HL5ESK-UZY7RRG-7BTW3AU',
                        name: 'macbookpro-docusign-2025',
                    },
                    'macbookpro-personal-2025': {
                        id: 'YFRAWME-SEF4QKB-GGUK25Q-ABHMASU-HIYZTRX-G3H2AD7-237IM24-R6SZHQG',
                        name: 'macbookpro-personal-2025',
                    },
                },
                folders: {
                    personal: {
                        id: 'org/personal',
                        path: '/var/syncthing/data/org/personal',
                        devices: [
                            'kubernetes-syncthing-mirror',
                            'macstudio-personal-2023',
                            'macbookpro-docusign-2025',
                            'macbookpro-personal-2025',
                        ],
                        type: 'sendreceive',
                    },
                    'notes-personal': {
                        id: 'notes/personal',
                        path: '/var/syncthing/data/notes/personal',
                        devices: [
                            'kubernetes-syncthing-mirror',
                            'macstudio-personal-2023',
                            'macbookpro-docusign-2025',
                            'macbookpro-personal-2025',
                        ],
                        type: 'sendreceive',
                    },
                    'notes-notedown': {
                        id: 'notes/.notedown',
                        path: '/var/syncthing/data/notes/notedown',
                        devices: [
                            'kubernetes-syncthing-mirror',
                            'macstudio-personal-2023',
                            'macbookpro-docusign-2025',
                            'macbookpro-personal-2025',
                        ],
                        type: 'sendreceive',
                    },
                },
            },
        },
        { ...opts, dependsOn: [namespace, ...storage.ready] },
    )

    return {
        namespace,
        syncthing,
        ready: [syncthing],
    }
}
