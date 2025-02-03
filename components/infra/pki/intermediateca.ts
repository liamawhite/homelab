import * as pulumi from '@pulumi/pulumi'
import { PrivateKey } from '@pulumi/tls/privateKey'
import { CertRequest } from '@pulumi/tls/certRequest'
import { LocallySignedCert } from '@pulumi/tls/locallySignedCert'
import { CertificateAuthorityCert } from './cacert'

export class IntermediateCertificateAuthority extends pulumi.ComponentResource implements CertificateAuthorityCert {
    readonly rootCert: pulumi.Input<string>

    readonly cert: pulumi.Input<string>

    readonly chain: pulumi.Input<string>

    readonly key: pulumi.Input<string>

    constructor(name: string, args: IntermediateCertificateAuthorityArgs) {
        super('homelab:pki:intermediate-ca', name)
        const opts = { parent: this }

        const pk = new PrivateKey(name, { algorithm: 'RSA' }, opts)
        const cr = new CertRequest(
            name,
            {
                privateKeyPem: pk.privateKeyPem,
                subject: { commonName: name },
            },
            opts,
        )
        const cert = new LocallySignedCert(
            name,
            {
                certRequestPem: cr.certRequestPem,
                caCertPem: args.parent.cert,
                caPrivateKeyPem: args.parent.key,
                validityPeriodHours: 24 * 365 * 100, // 100 years its non-prod
                isCaCertificate: true,
                allowedUses: ['cert_signing'],
            },
            opts,
        )

        this.cert = cert.certPem
        this.rootCert = args.parent.rootCert
        this.chain = pulumi.interpolate`${this.cert}${args.parent.chain}`
        this.key = pk.privateKeyPem
    }

    public issueIntermediateCA = (name: string): IntermediateCertificateAuthority =>
        new IntermediateCertificateAuthority(name, { parent: this })
}

interface IntermediateCertificateAuthorityArgs {
    parent: CertificateAuthorityCert
}
