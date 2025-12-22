import * as pulumi from '@pulumi/pulumi'

export interface CertificateAuthorityCert {
    rootCert: pulumi.Input<string>
    cert: pulumi.Input<string>
    chain: pulumi.Input<string>
    key: pulumi.Input<string>
}
