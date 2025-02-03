import { RootCertificateAuthority } from '../components/infra/pki/rootca'
import { CertManager } from '../components/kubernetes/certmanager'
import { PrivateClusterIssuer } from '../components/kubernetes/certmanager/privateissuer'
import { configureCluster } from './cluster'

export function configurePki({
    provider,
}: ReturnType<typeof configureCluster>) {
    const ca = new RootCertificateAuthority('Homelab Root CA')

    const certmanager = new CertManager('cert-manager', {}, { provider })
    const issuer = new PrivateClusterIssuer(
        'privateissuer',
        {
            namespace: certmanager.namespace,
            ca: ca.issueIntermediateCA('Homelab Cert Manager CA'),
        },
        { provider, parent: certmanager },
    )

    return {
        ca,
        issuer,
        ready: [ca, certmanager, issuer],
    }
}
