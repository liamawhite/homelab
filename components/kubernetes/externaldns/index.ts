import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { versions } from "../../../.versions";

export class ExternalDns extends pulumi.ComponentResource {
    readonly namespace: pulumi.Output<string>

    constructor(name: string, args: ExternalDnsArgs, opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }) {
        super("homelab:kubernetes:external-dns", name, {}, opts)
        const localOpts = { ...opts, parent: this }

        const namespace = new k8s.core.v1.Namespace(name, {
            metadata: { name: "external-dns" },
        }, localOpts)
        this.namespace = namespace.metadata.name

        const secret = new k8s.core.v1.Secret(name, {
            metadata: { namespace: namespace.metadata.name },
            stringData: {
                "EXTERNAL_DNS_PIHOLE_PASSWORD": args.pihole.password,
            },
        }, localOpts)

        const role = new k8s.rbac.v1.ClusterRole(name, {
            rules: [
                {
                    apiGroups: [""],
                    resources: ["services", "endpoints", "pods"],
                    verbs: ["get", "watch", "list"],
                },
                {
                    apiGroups: ["extensions", "networking.k8s.io"],
                    resources: ["ingresses"],
                    verbs: ["get", "watch", "list"],
                },
                {
                    apiGroups: [""],
                    resources: ["nodes"],
                    verbs: ["list", "watch"],
                },
            ],
        }, localOpts)

        const sa = new k8s.core.v1.ServiceAccount(name, {
            metadata: { namespace: namespace.metadata.name },
        }, localOpts)

        const binding = new k8s.rbac.v1.ClusterRoleBinding(name, {
            roleRef: {
                apiGroup: "rbac.authorization.k8s.io",
                kind: role.kind,
                name: role.metadata.name,
            },
            subjects: [
                {
                    kind: sa.kind,
                    name: sa.metadata.name,
                    namespace: namespace.metadata.name,
                },
            ],
        }, localOpts)

        const deploy = new k8s.apps.v1.Deployment(name, {
            metadata: { namespace: namespace.metadata.name },
            spec: {
                selector: { matchLabels: { app: "external-dns" } },
                strategy: { type: "Recreate" },
                template: {
                    metadata: { labels: { app: "external-dns" } },
                    spec: {
                        containers: [{
                            args: [
                                "--source=service",
                                "--source=ingress",
                                "--registry=noop", // pihole doesnt support txt records
                                // "--policy=upsert-only",
                                "--provider=pihole",
                                pulumi.interpolate`--pihole-server=${args.pihole.web}`,
                            ],
                            envFrom: [{
                                secretRef: { name: secret.metadata.name },
                            }],
                            image: `registry.k8s.io/external-dns/external-dns:v${versions.externalDns}`,
                            name: "external-dns",
                        }],
                        securityContext: {
                            fsGroup: 65534,
                        },
                        serviceAccountName: sa.metadata.name,
                    },
                },
            },
        }, localOpts);

        this.registerOutputs({ secret, sa, role, binding, deploy })
    }
}

export interface ExternalDnsArgs {
    pihole: {
        web: pulumi.Input<string>
        password: pulumi.Input<string>
    }
}

