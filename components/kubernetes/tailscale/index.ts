import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";

export class TailscaleOperator extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(name: string, args: TailscaleOperatorArgs, opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }) {
        super("homelab:kubernetes:tailscaleoperator", name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(name, {
            metadata: { name: "tailscale-system" },
        }, localOpts)
        this.namespace = namespace.metadata.name

        const install = new k8s.helm.v4.Chart(name, {
            namespace: namespace.metadata.name,
            chart: "tailscale-operator",
            version: "1.78.3",
            repositoryOpts: { repo: "https://pkgs.tailscale.com/helmcharts" },
            values: {
                oauth: {
                    clientId: args.clientId,
                    clientSecret: args.clientSecret,
                },
                operatorConfig: {
                    hostname: args.hostname,
                }
            },
        }, localOpts)

        this.registerOutputs({ install: install.resources })
    }
}

export interface TailscaleOperatorArgs {
    hostname: pulumi.Input<string>
    clientId: pulumi.Input<string>
    clientSecret: pulumi.Input<string>
}

