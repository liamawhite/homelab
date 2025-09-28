import * as pulumi from '@pulumi/pulumi'
import * as k8s from '@pulumi/kubernetes'
import { gateway } from '../istio/crds/gatewayapi'
import { Certificate } from '../certmanager/crds/cert_manager/v1'
import { cert_manager as certmanager } from '../certmanager/crds/types/input'
import { versions } from '../../../.versions'

export class Grafana extends pulumi.ComponentResource {
    constructor(
        name: string,
        args: GrafanaArgs,
        opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider },
    ) {
        super('homelab:kubernetes:grafana', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        // ConfigMap for dashboard definitions
        const dashboardConfigMap = new k8s.core.v1.ConfigMap(
            `${name}-dashboards`,
            {
                metadata: {
                    name: 'grafana-dashboards',
                    namespace: args.namespace,
                    labels: {
                        app: 'grafana',
                        component: 'dashboards',
                    },
                },
                data: args.dashboards,
            },
            localOpts,
        )

        // ConfigMap for Grafana configuration
        const configMap = new k8s.core.v1.ConfigMap(
            `${name}-config`,
            {
                metadata: {
                    name: 'grafana-config',
                    namespace: args.namespace,
                    labels: {
                        app: 'grafana',
                        component: 'config',
                    },
                },
                data: pulumi.all([args.prometheus.namespace]).apply(([prometheusNamespace]) => ({
                    'grafana.ini': `[analytics]
check_for_updates = true

[grafana_net]
url = https://grafana.net

[log]
mode = console

[paths]
data = /var/lib/grafana/
logs = /var/log/grafana
plugins = /var/lib/grafana/plugins
provisioning = /etc/grafana/provisioning

[server]
domain = ${args.web.hostname}
root_url = https://%(domain)s

[auth.anonymous]
enabled = true
org_role = Admin

[security]
admin_user = admin
disable_initial_admin_creation = true

[users]
allow_sign_up = false`,

                    'datasources.yml': `apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    uid: prometheus
    access: proxy
    url: http://prometheus-k8s.${prometheusNamespace}:9090
    isDefault: true
    jsonData:
      timeInterval: 30s`,

                    'dashboards.yml': `apiVersion: 1

providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards`,
                })),
            },
            localOpts,
        )

        // Deployment for Grafana
        const deployment = new k8s.apps.v1.Deployment(
            name,
            {
                metadata: {
                    name: 'grafana',
                    namespace: args.namespace,
                    labels: { app: 'grafana' },
                },
                spec: {
                    replicas: 1,
                    selector: {
                        matchLabels: { app: 'grafana' },
                    },
                    template: {
                        metadata: {
                            labels: { app: 'grafana' },
                        },
                        spec: {
                            securityContext: {
                                fsGroup: 472,
                                runAsUser: 472,
                                runAsGroup: 472,
                                runAsNonRoot: true,
                            },
                            containers: [
                                {
                                    name: 'grafana',
                                    image: `grafana/grafana:${versions.grafana}`,
                                    ports: [
                                        {
                                            containerPort: 3000,
                                            name: 'web',
                                            protocol: 'TCP',
                                        },
                                    ],
                                    env: [
                                        {
                                            name: 'GF_SECURITY_ADMIN_USER',
                                            value: 'admin',
                                        },
                                    ],
                                    livenessProbe: {
                                        httpGet: {
                                            path: '/api/health',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 30,
                                        timeoutSeconds: 30,
                                        periodSeconds: 10,
                                        successThreshold: 1,
                                        failureThreshold: 3,
                                    },
                                    readinessProbe: {
                                        httpGet: {
                                            path: '/api/health',
                                            port: 'web',
                                            scheme: 'HTTP',
                                        },
                                        initialDelaySeconds: 5,
                                        timeoutSeconds: 3,
                                        periodSeconds: 5,
                                        successThreshold: 1,
                                        failureThreshold: 3,
                                    },
                                    resources: {
                                        requests: {
                                            cpu: '100m',
                                            memory: '128Mi',
                                        },
                                        limits: {
                                            cpu: '500m',
                                            memory: '512Mi',
                                        },
                                    },
                                    volumeMounts: [
                                        {
                                            name: 'config',
                                            mountPath: '/etc/grafana/grafana.ini',
                                            subPath: 'grafana.ini',
                                            readOnly: true,
                                        },
                                        {
                                            name: 'config',
                                            mountPath:
                                                '/etc/grafana/provisioning/datasources/datasources.yml',
                                            subPath: 'datasources.yml',
                                            readOnly: true,
                                        },
                                        {
                                            name: 'config',
                                            mountPath:
                                                '/etc/grafana/provisioning/dashboards/dashboards.yml',
                                            subPath: 'dashboards.yml',
                                            readOnly: true,
                                        },
                                        {
                                            name: 'dashboards',
                                            mountPath: '/var/lib/grafana/dashboards',
                                            readOnly: true,
                                        },
                                        {
                                            name: 'data',
                                            mountPath: '/var/lib/grafana',
                                        },
                                    ],
                                },
                            ],
                            volumes: [
                                {
                                    name: 'config',
                                    configMap: {
                                        name: configMap.metadata.name,
                                    },
                                },
                                {
                                    name: 'dashboards',
                                    configMap: {
                                        name: dashboardConfigMap.metadata.name,
                                    },
                                },
                                {
                                    name: 'data',
                                    persistentVolumeClaim: {
                                        claimName: 'grafana-data',
                                    },
                                },
                            ],
                        },
                    },
                },
            },
            localOpts,
        )

        // PersistentVolumeClaim for Grafana data
        const pvc = new k8s.core.v1.PersistentVolumeClaim(
            `${name}-data`,
            {
                metadata: {
                    name: 'grafana-data',
                    namespace: args.namespace,
                    labels: { app: 'grafana' },
                },
                spec: {
                    accessModes: ['ReadWriteOnce'],
                    storageClassName: args.storage.storageClassName,
                    resources: {
                        requests: {
                            storage: args.storage.size,
                        },
                    },
                },
            },
            localOpts,
        )

        // Service for Grafana
        const service = new k8s.core.v1.Service(
            `${name}-service`,
            {
                metadata: {
                    name: 'grafana',
                    namespace: args.namespace,
                    labels: { app: 'grafana' },
                },
                spec: {
                    type: 'ClusterIP',
                    ports: [
                        {
                            name: 'web',
                            port: 3000,
                            protocol: 'TCP',
                            targetPort: 'web',
                        },
                    ],
                    selector: { app: 'grafana' },
                },
            },
            localOpts,
        )

        // TLS Certificate
        const cert = new Certificate(
            `${name}-cert`,
            {
                metadata: {
                    name: 'grafana-cert',
                    namespace: args.namespace,
                },
                spec: {
                    dnsNames: [args.web.hostname],
                    issuerRef: args.web.issuer,
                    secretName: 'grafana-tls',
                },
            },
            localOpts,
        )

        // Istio Gateway
        const gw = new gateway.v1.Gateway(
            `${name}-gateway`,
            {
                metadata: {
                    name: 'grafana-gateway',
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
                                certificateRefs: [{ name: cert.spec.secretName }],
                            },
                            allowedRoutes: { namespaces: { from: 'Same' } },
                        },
                    ],
                },
            },
            localOpts,
        )

        // HTTP to HTTPS redirect
        const httpRedirect = new gateway.v1.HTTPRoute(
            `${name}-http-redirect`,
            {
                metadata: {
                    name: 'grafana-http-redirect',
                    namespace: args.namespace,
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

        // HTTPS HTTPRoute
        const httpRoute = new gateway.v1.HTTPRoute(
            `${name}-https-route`,
            {
                metadata: {
                    name: 'grafana-https-route',
                    namespace: args.namespace,
                },
                spec: {
                    hostnames: [args.web.hostname],
                    parentRefs: [{ name: gw.metadata.name, sectionName: 'https' }],
                    rules: [
                        {
                            backendRefs: [{ name: service.metadata.name, port: 3000 }],
                        },
                    ],
                },
            },
            localOpts,
        )

        this.registerOutputs({
            configMap,
            dashboardConfigMap,
            deployment,
            pvc,
            service,
            cert,
            gw,
            httpRedirect,
            httpRoute,
        })
    }
}

export interface GrafanaArgs {
    namespace: pulumi.Input<string>
    dashboards: Record<string, string>
    storage: {
        size: string
        storageClassName: pulumi.Input<string>
    }
    prometheus: {
        namespace: pulumi.Input<string>
    }
    web: {
        hostname: string
        issuer: certmanager.v1.CertificateSpecIssuerRef
    }
}
