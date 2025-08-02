import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { versions } from '../../../.versions'

export class Longhorn extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(
        name: string,
        args: LonghornArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:longhorn', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(
            name,
            {
                metadata: { name: 'longhorn-system' },
            },
            localOpts,
        )
        this.namespace = namespace.metadata.name

        const install = new k8s.helm.v4.Chart(
            name,
            {
                namespace: namespace.metadata.name,
                chart: 'longhorn',
                version: versions.longhorn,
                repositoryOpts: { repo: 'https://charts.longhorn.io' },
            },
            localOpts,
        )

        this.registerOutputs({ install: install.resources })
    }
}

export interface LonghornArgs {}
