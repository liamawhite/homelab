import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { versions } from '../../../.versions'
import { GATEWAY_API_URL } from './gen/gatewayapi'

export class Istio extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(
        name: string,
        args: IstioArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:istio', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(
            name,
            {
                metadata: { name: 'istio-system' },
            },
            localOpts,
        )
        this.namespace = namespace.metadata.name

        const repositoryOpts = {
            repo: 'https://istio-release.storage.googleapis.com/charts',
        }

        const crds = new k8s.helm.v4.Chart(
            `${name}-crds`,
            {
                namespace: namespace.metadata.name,
                chart: 'base',
                version: versions.istio,
                repositoryOpts,
                values: {},
            },
            localOpts,
        )

        const gatewayApi = new k8s.yaml.ConfigFile(
            `${name}-gateway-api`,
            {
                file: GATEWAY_API_URL,
            },
            localOpts,
        )

        const istiod = new k8s.helm.v4.Chart(
            `${name}-istiod`,
            {
                namespace: namespace.metadata.name,
                chart: 'istiod',
                version: versions.istio,
                repositoryOpts,
                values: {
                    profile: 'ambient',
                    pilot: {
                        resources: {
                            limits: {
                                cpu: '200m',
                                memory: '128Mi',
                            },
                            requests: {
                                cpu: '20m',
                                memory: '64Mi',
                            },
                        },
                    },
                },
            },
            localOpts,
        )

        const cni = new k8s.helm.v4.Chart(
            `${name}-cni`,
            {
                namespace: namespace.metadata.name,
                chart: 'cni',
                version: versions.istio,
                repositoryOpts,
                values: {
                    profile: 'ambient',
                    global: {
                        platform: 'k3s',
                    },
                    // still had to set these manually for some reason
                    // https://github.com/k3s-io/k3s/issues/11076
                    cni: {
                        cniConfDir: '/var/lib/rancher/k3s/agent/etc/cni/net.d',
                        cniBinDir: '/var/lib/rancher/k3s/data/cni',
                        resources: {
                            limits: {
                                cpu: '100m',
                                memory: '64Mi',
                            },
                            requests: {
                                cpu: '10m',
                                memory: '32Mi',
                            },
                        },
                    },
                },
            },
            localOpts,
        )

        const ztunnel = new k8s.helm.v4.Chart(
            `${name}-ztunnel`,
            {
                namespace: namespace.metadata.name,
                chart: 'ztunnel',
                version: versions.istio,
                repositoryOpts,
                values: {
                    resources: {
                        limits: {
                            cpu: '200m',
                            memory: '128Mi',
                        },
                        requests: {
                            cpu: '20m',
                            memory: '96Mi',
                        },
                    },
                },
            },
            localOpts,
        )

        this.registerOutputs({
            crds: crds.resources,
            gatewayApi: gatewayApi.resources,
            istiod: istiod.resources,
            cni: cni.resources,
        })
    }
}

export interface IstioArgs {}
