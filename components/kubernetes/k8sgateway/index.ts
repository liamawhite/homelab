import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { versions } from '../../../.versions'

export class K8sGateway extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>
    readonly serviceIP: pulumi.Output<string>

    constructor(
        name: string,
        args: K8sGatewayArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:k8s-gateway', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        this.namespace = pulumi.output(args.namespace)

        const install = new k8s.helm.v4.Chart(
            name,
            {
                namespace: this.namespace,
                chart: 'k8s-gateway',
                version: versions.k8sGateway,
                repositoryOpts: { repo: 'https://k8s-gateway.github.io/k8s_gateway' },
                values: {
                    domain: args.domain,
                    watchedResources: ['HTTPRoute'],
                    gatewayClass: 'istio',
                    service: {
                        type: 'ClusterIP',
                        clusterIP: args.clusterIP,
                        port: 53,
                    },
                    rbac: {
                        create: true,
                    },
                },
            },
            localOpts,
        )

        this.serviceIP = pulumi.output(args.clusterIP)

        this.registerOutputs({ install: install.resources, serviceIP: this.serviceIP })
    }
}

export interface K8sGatewayArgs {
    domain: string
    clusterIP: string
    namespace: pulumi.Input<string>
}
