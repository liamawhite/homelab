import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { versions } from '../../../.versions'

export class MetalLb extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(
        name: string,
        args: MetalLbArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:metallb', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(
            name,
            {
                metadata: { name: 'metallb-system' },
            },
            localOpts,
        )
        this.namespace = namespace.metadata.name

        const install = new k8s.helm.v4.Chart(
            name,
            {
                namespace: namespace.metadata.name,
                chart: 'metallb',
                version: versions.metallb,
                repositoryOpts: { repo: 'https://metallb.github.io/metallb' },
                values: {
                    controller: {
                        resources: {
                            limits: {
                                cpu: '100m',
                                memory: '128Mi',
                            },
                            requests: {
                                cpu: '10m',
                                memory: '64Mi',
                            },
                        },
                    },
                    speaker: {
                        resources: {
                            limits: {
                                cpu: '200m',
                                memory: '128Mi',
                            },
                            requests: {
                                cpu: '50m',
                                memory: '64Mi',
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

export interface MetalLbArgs {}
