import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import * as random from '@pulumi/random'
import * as labels from '../istio/labels'
import { gateway } from '../istio/crds/gatewayapi'
import { hostname } from '../externaldns/annotations'
import { Certificate } from '../certmanager/crds/cert_manager/v1'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'
import { versions } from '../../../.versions'

export class Pihole extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    readonly localAddress: pulumi.Output<string>
    readonly password: pulumi.Output<string>

    constructor(
        name: string,
        args: PiholeArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:pihole', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const ns = new k8s.core.v1.Namespace(
            name,
            {
                metadata: {
                    name: 'pihole',
                    labels: labels.namespace.enableAmbient,
                },
            },
            localOpts,
        )
        this.namespace = ns.metadata.name

        this.password = new random.RandomPassword(
            name,
            {
                length: 16,
                special: false,
            },
            { parent: this },
        ).result

        const secret = new k8s.core.v1.Secret(
            name,
            {
                metadata: { namespace: ns.metadata.name },
                stringData: {
                    password: this.password,
                },
            },
            localOpts,
        )

        const deploy = new k8s.apps.v1.Deployment(
            name,
            {
                metadata: {
                    name: 'pihole',
                    namespace: this.namespace,
                    labels: { app: 'pihole' },
                },
                spec: {
                    replicas: 1, // keep at one for now otherwise we will need session affinity
                    selector: {
                        matchLabels: { app: 'pihole' },
                    },
                    strategy: {
                        type: 'RollingUpdate',
                        rollingUpdate: { maxSurge: 1, maxUnavailable: 1 },
                    },
                    template: {
                        metadata: { labels: { app: 'pihole' } },
                        spec: {
                            containers: [
                                {
                                    env: [
                                        {
                                            name: 'WEB_PORT',
                                            value: '8080',
                                        },
                                        {
                                            name: 'WEBPASSWORD',
                                            valueFrom: {
                                                secretKeyRef: {
                                                    key: 'password',
                                                    name: secret.metadata.name,
                                                },
                                            },
                                        },
                                        {
                                            name: 'PIHOLE_DNS_',
                                            value: '8.8.8.8;8.8.4.4',
                                        },
                                    ],
                                    image: `pihole/pihole:${versions.pihole}`,
                                    imagePullPolicy: 'IfNotPresent',
                                    livenessProbe: {
                                        failureThreshold: 10,
                                        httpGet: {
                                            path: '/admin/index.php',
                                            port: 'http',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 60,
                                        timeoutSeconds: 5,
                                    },
                                    name: 'pihole',
                                    ports: [
                                        {
                                            containerPort: 8080,
                                            name: 'http',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 53,
                                            name: 'dns',
                                            protocol: 'TCP',
                                        },
                                        {
                                            containerPort: 53,
                                            name: 'dns-udp',
                                            protocol: 'UDP',
                                        },
                                        {
                                            containerPort: 67,
                                            name: 'client-udp',
                                            protocol: 'UDP',
                                        },
                                    ],
                                    readinessProbe: {
                                        failureThreshold: 10,
                                        httpGet: {
                                            path: '/admin/index.php',
                                            port: 'http',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 20,
                                        timeoutSeconds: 5,
                                    },
                                    resources: {},
                                    securityContext: { privileged: false },
                                    volumeMounts: [
                                        {
                                            mountPath: '/etc/pihole',
                                            name: 'config',
                                        },
                                        // {
                                        // mountPath: "/etc/dnsmasq.d/02-custom.conf",
                                        // name: "custom-dnsmasq",
                                        // subPath: "02-custom.conf",
                                        // },
                                        // {
                                        // mountPath: "/etc/addn-hosts",
                                        // name: "custom-dnsmasq",
                                        // subPath: "addn-hosts",
                                        // },
                                    ],
                                },
                            ],
                            hostNetwork: false,
                            volumes: [
                                {
                                    emptyDir: {},
                                    name: 'config',
                                },
                                // {
                                // configMap: {
                                // defaultMode: 420,
                                // name: "pihole-custom-dnsmasq",
                                // },
                                // name: "custom-dnsmasq",
                                // },
                            ],
                        },
                    },
                },
            },
            localOpts,
        )

        const dns = new k8s.core.v1.Service(
            `${name}-dns`,
            {
                metadata: {
                    name: 'pihole-dns',
                    namespace: this.namespace,
                    labels: deploy.metadata.labels,
                    annotations: args.dns?.annotations,
                },
                spec: {
                    type: 'LoadBalancer',
                    ports: [
                        {
                            name: 'dns-udp',
                            port: 53,
                            protocol: 'UDP',
                            targetPort: 'dns-udp',
                        },
                        {
                            name: 'dns-tcp',
                            port: 53,
                            protocol: 'TCP',
                            targetPort: 'dns',
                        },
                    ],
                    selector: deploy.spec.template.metadata.labels,
                },
            },
            localOpts,
        )

        const http = new k8s.core.v1.Service(
            `${name}-http`,
            {
                metadata: {
                    name: 'pihole-http',
                    namespace: this.namespace,
                    labels: deploy.metadata.labels,
                },
                spec: {
                    ports: [
                        {
                            name: 'http',
                            port: 8080,
                            protocol: 'TCP',
                            targetPort: 'http',
                        },
                    ],
                    selector: deploy.spec.template.metadata.labels,
                },
            },
            localOpts,
        )

        this.localAddress = pulumi.interpolate`http://${http.metadata.name}.${this.namespace}.svc.cluster.local:8080`

        const cert = new Certificate(
            name,
            {
                metadata: { namespace: this.namespace },
                spec: {
                    dnsNames: [args.web.hostname],
                    issuerRef: args.web.issuer,
                    secretName: 'pihole-cert',
                },
            },
            localOpts,
        )

        const gw = new gateway.v1.Gateway(
            name,
            {
                metadata: {
                    name: 'pihole-webui',
                    namespace: this.namespace,
                    annotations: hostname(args.web.hostname),
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
                    name: 'pihole-webui-httpredirect',
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
                    name: 'pihole-webui',
                    namespace: this.namespace,
                },
                spec: {
                    parentRefs: [{ name: gw.metadata.name, sectionName: 'https' }],
                    rules: [
                        // redirect root to /admin because I forget this all the time
                        {
                            matches: [{ path: { type: 'Exact', value: '/' } }],
                            filters: [
                                {
                                    type: 'RequestRedirect',
                                    requestRedirect: {
                                        path: {
                                            type: 'ReplaceFullPath',
                                            replaceFullPath: '/admin',
                                        },
                                        statusCode: 301,
                                    },
                                },
                            ],
                        },
                        {
                            backendRefs: [{ name: http.metadata.name, port: 8080 }],
                        },
                    ],
                },
            },
            localOpts,
        )

        this.registerOutputs({
            deploy,
            dns,
            http,
            httpRoute,
            httpRedirect,
            cert,
        })
    }
}

export interface PiholeArgs {
    web: {
        hostname: string
        issuer: certmanager.v1.CertificateSpecIssuerRef
    }
    dns?: {
        annotations?: {
            [key: string]: pulumi.Input<string>
        }
    }
}
