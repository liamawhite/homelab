import * as pulumi from '@pulumi/pulumi'
import { PrivateKey } from '@pulumi/tls/privateKey'
import { SelfSignedCert } from '@pulumi/tls/selfSignedCert'
import { CertificateAuthorityCert } from './cacert'
import { IntermediateCertificateAuthority } from './intermediateca'

export class RootCertificateAuthority
    extends pulumi.ComponentResource
    implements CertificateAuthorityCert
{
    readonly rootCert: pulumi.Input<string>

    readonly cert: pulumi.Input<string>

    readonly chain: pulumi.Input<string>

    readonly key: pulumi.Input<string>

    readonly algorithm: pulumi.Input<string>

    constructor(name: string) {
        super('homelab:pki:root-ca', name)
        const opts = { parent: this }

        const pk = new PrivateKey(name, { algorithm: 'RSA' }, opts)
        const cert = new SelfSignedCert(
            name,
            {
                allowedUses: ['cert_signing'],
                subject: { commonName: name },
                isCaCertificate: true,
                privateKeyPem: pk.privateKeyPem,
                validityPeriodHours: 24 * 365 * 100, // 100 years
            },
            opts,
        )

        this.cert = cert.certPem
        this.rootCert = cert.certPem
        this.chain = this.cert
        this.key = pk.privateKeyPem
        this.algorithm = cert.keyAlgorithm
    }
    public issueIntermediateCA = (
        name: string,
    ): IntermediateCertificateAuthority =>
        new IntermediateCertificateAuthority(name, { parent: this })
}
