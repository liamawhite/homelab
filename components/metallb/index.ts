import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";

export class MetalLb extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(name: string, args: MetalLbArgs, opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }) {
        super("homelab:kubernetes:metallb", name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(`${name}-metallb`, {
            metadata: { name: "metallb-system" },
        }, localOpts)
        this.namespace = namespace.metadata.name

        const install = new k8s.helm.v4.Chart(`${name}-metallb`, {
            namespace: namespace.metadata.name,
            chart: "metallb",
            version: "0.14.9",
            repositoryOpts: { repo: "https://metallb.github.io/metallb" },
        }, localOpts)

        this.registerOutputs({ install: install.resources })
    }
}

export interface MetalLbArgs {
    addresses?: string[]
}

