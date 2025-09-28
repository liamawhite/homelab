import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { gateway } from '../istio/crds/gatewayapi'
import { Certificate } from '../certmanager/crds/cert_manager/v1'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'

export class Gateway extends pulumi.ComponentResource {
    readonly certificate: Certificate
    readonly gateway: gateway.v1.Gateway
    readonly httpRedirect: gateway.v1.HTTPRoute
    readonly httpRoute: gateway.v1.HTTPRoute
    readonly tailscaleIngress?: k8s.networking.v1.Ingress

    constructor(
        name: string,
        args: GatewayArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:gateway', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // TLS Certificate
        this.certificate = new Certificate(
            `${name}-cert`,
            {
                metadata: {
                    name: `${name}-cert`,
                    namespace: args.namespace,
                },
                spec: {
                    dnsNames: [args.hostname],
                    issuerRef: args.issuer,
                    secretName: `${name}-tls`,
                },
            },
            localOpts,
        )

        // Gateway API Gateway
        this.gateway = new gateway.v1.Gateway(
            `${name}-gateway`,
            {
                metadata: {
                    name: `${name}-gateway`,
                    namespace: args.namespace,
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
                                certificateRefs: [{ name: this.certificate.spec.secretName }],
                            },
                            allowedRoutes: { namespaces: { from: 'Same' } },
                        },
                    ],
                },
            },
            localOpts,
        )

        // HTTP to HTTPS redirect
        this.httpRedirect = new gateway.v1.HTTPRoute(
            `${name}-http-redirect`,
            {
                metadata: {
                    name: `${name}-http-redirect`,
                    namespace: args.namespace,
                },
                spec: {
                    parentRefs: [{ name: this.gateway.metadata.name, sectionName: 'http' }],
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

        // HTTPS HTTPRoute
        this.httpRoute = new gateway.v1.HTTPRoute(
            `${name}-https-route`,
            {
                metadata: {
                    name: `${name}-https-route`,
                    namespace: args.namespace,
                },
                spec: {
                    hostnames: [args.hostname],
                    parentRefs: [{ name: this.gateway.metadata.name, sectionName: 'https' }],
                    rules: [
                        {
                            backendRefs: [{ name: args.serviceName, port: args.servicePort }],
                        },
                    ],
                },
            },
            localOpts,
        )

        // Optional Tailscale Ingress
        if (args.tailscale?.enabled) {
            this.tailscaleIngress = new k8s.networking.v1.Ingress(
                `${name}-tailscale-ingress`,
                {
                    metadata: {
                        name: `${name}-tailscale`,
                        namespace: args.namespace,
                        annotations: args.tailscale.hostname
                            ? { 'tailscale.com/hostname': args.tailscale.hostname }
                            : undefined,
                    },
                    spec: {
                        ingressClassName: 'tailscale',
                        defaultBackend: {
                            service: {
                                name: args.serviceName,
                                port: {
                                    number: args.servicePort,
                                },
                            },
                        },
                        tls: [
                            {
                                hosts: [args.tailscale.hostname || args.hostname],
                            },
                        ],
                    },
                },
                localOpts,
            )
        }

        this.registerOutputs({
            certificate: this.certificate,
            gateway: this.gateway,
            httpRedirect: this.httpRedirect,
            httpRoute: this.httpRoute,
            tailscaleIngress: this.tailscaleIngress,
        })
    }
}

export interface GatewayArgs {
    namespace: pulumi.Input<string>
    hostname: string
    serviceName: pulumi.Input<string>
    servicePort: number
    issuer: certmanager.v1.CertificateSpecIssuerRef
    tailscale?: {
        enabled: boolean
        hostname?: string
    }
}
