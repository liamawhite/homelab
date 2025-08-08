import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { versions } from '../../../.versions'

export class CertManager extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(
        name: string,
        args: CertManagerArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:cert-manager', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(
            name,
            {
                metadata: { name: 'cert-manager' },
            },
            localOpts,
        )
        this.namespace = namespace.metadata.name

        const install = new k8s.helm.v4.Chart(
            name,
            {
                namespace: namespace.metadata.name,
                chart: 'cert-manager',
                version: versions.certManager,
                repositoryOpts: { repo: 'https://charts.jetstack.io' },
                values: {
                    crds: { enabled: true },
                    resources: {
                        limits: {
                            cpu: '50m',
                            memory: '64Mi',
                        },
                        requests: {
                            cpu: '5m',
                            memory: '32Mi',
                        },
                    },
                    cainjector: {
                        resources: {
                            limits: {
                                cpu: '100m',
                                memory: '128Mi',
                            },
                            requests: {
                                cpu: '50m',
                                memory: '64Mi',
                            },
                        },
                    },
                    webhook: {
                        resources: {
                            limits: {
                                cpu: '50m',
                                memory: '64Mi',
                            },
                            requests: {
                                cpu: '5m',
                                memory: '32Mi',
                            },
                        },
                    },
                },
            },
            localOpts,
        )

        this.registerOutputs({ install: install.resources })
    }
}

export interface CertManagerArgs {}
