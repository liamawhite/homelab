import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import { ClusterIssuer } from "./crds/cert_manager/v1";
import { cert_manager } from "./crds/types/input";

export class PrivateClusterIssuer extends pulumi.ComponentResource {
    readonly name: pulumi.Input<string>
    readonly kind: string
    readonly group: string

    constructor(name: string, args: PrivateClusterIssuerArgs, opts: pulumi.ComponentResourceOptions & { provider: k8s.Provider }) {
        super("homelab:kubernetes:private-issuer", name, {}, opts)
        const localOpts = { ...opts, parent: this }

        this.kind = "ClusterIssuer"
        this.group = "cert-manager.io"

        const secret = new k8s.core.v1.Secret(name, {
            metadata: { namespace: args.namespace, },
            stringData: {
                "tls.key": args.ca.key,
                "tls.crt": args.ca.chain,
            },
        }, localOpts)

        const issuer = new ClusterIssuer(name, {
            spec: {
                ca: {
                    secretName: secret.metadata.name,
                },
            },
        }, localOpts)

        this.name = issuer.metadata.name

        this.registerOutputs({ issuer, secret })
    }

    issuerRef(): cert_manager.v1.CertificateSpecIssuerRef {
        return {
            name: this.name,
            kind: this.kind,
            group: this.group,
        }
    }
}

export interface PrivateClusterIssuerArgs {
    namespace: pulumi.Input<string>
    ca: {
        key: pulumi.Input<string>
        chain: pulumi.Input<string>
    }

}
