import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { gateway } from '../istio/crds/gatewayapi'
import { Certificate } from '../certmanager/crds/cert_manager/v1'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'
import { versions } from '../../../.versions'
import * as labels from '../istio/labels'

export class Longhorn extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>
    readonly defaultStorageClass: k8s.storage.v1.StorageClass

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
                metadata: {
                    name: 'longhorn-system',
                    labels: labels.namespace.enableAmbient,
                },
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

        const defaultStorageClass = k8s.storage.v1.StorageClass.get(
            'longhorn-default',
            'longhorn',
            { ...localOpts, dependsOn: [install] },
        )
        this.defaultStorageClass = defaultStorageClass

        const cert = new Certificate(
            name,
            {
                metadata: { namespace: this.namespace },
                spec: {
                    dnsNames: [args.web.hostname],
                    issuerRef: args.web.issuer,
                    secretName: 'longhorn-cert',
                },
            },
            localOpts,
        )

        const gw = new gateway.v1.Gateway(
            name,
            {
                metadata: {
                    name: 'longhorn-webui',
                    namespace: this.namespace,
                },
                spec: {
                    gatewayClassName: 'istio',
                    listeners: [
                        {
                            name: 'http',
                            port: 80,
                            protocol: 'HTTP',
                        },
                        {
                            name: 'https',
                            port: 443,
                            protocol: 'HTTPS',
                            tls: {
                                mode: 'Terminate',
                                certificateRefs: [{ name: cert.spec.secretName }],
                            },
                            allowedRoutes: { namespaces: { from: 'Same' } },
                        },
                    ],
                },
            },
            localOpts,
        )

        const httpRedirect = new gateway.v1.HTTPRoute(
            `${name}-redirect`,
            {
                metadata: {
                    name: 'longhorn-webui-httpredirect',
                    namespace: this.namespace,
                },
                spec: {
                    parentRefs: [{ name: gw.metadata.name, sectionName: 'http' }],
                    rules: [
                        {
                            filters: [
                                {
                                    type: 'RequestRedirect',
                                    requestRedirect: {
                                        scheme: 'https',
                                        statusCode: 301,
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
            localOpts,
        )

        const httpRoute = new gateway.v1.HTTPRoute(
            name,
            {
                metadata: {
                    name: 'longhorn-webui',
                    namespace: this.namespace,
                },
                spec: {
                    hostnames: [args.web.hostname],
                    parentRefs: [{ name: gw.metadata.name, sectionName: 'https' }],
                    rules: [
                        {
                            backendRefs: [{ name: 'longhorn-frontend', port: 80 }],
                        },
                    ],
                },
            },
            localOpts,
        )

        this.registerOutputs({
            install: install.resources,
            defaultStorageClass,
            cert,
            gw,
            httpRedirect,
            httpRoute,
        })
    }
}

export interface LonghornArgs {
    web: {
        hostname: string
        issuer: certmanager.v1.CertificateSpecIssuerRef
    }
}
