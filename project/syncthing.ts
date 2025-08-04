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
                    'macbookpro-personal-2018': {
                        id: 'OIDLC3D-EX3JH4E-FKEE2X5-YKGVYM4-7QLPPJQ-6ELD5T6-2GQB473-N6CRSAA',
                        name: 'macbookpro-personal-2018',
                    },
                    'macbookpro-docusign-2025': {
                        id: 'VNBUOSV-NBA7BXM-FCQ2KWS-5YIJNTQ-ZXP2LI6-6HL5ESK-UZY7RRG-7BTW3AU',
                        name: 'macbookpro-docusign-2025',
                    },
                },
                folders: {
                    personal: {
                        id: 'org/personal',
                        path: '/var/syncthing/data/org/personal',
                        devices: ['kubernetes-syncthing-mirror', 'macstudio-personal-2023', 'macbookpro-personal-2018', 'macbookpro-docusign-2025'],
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
