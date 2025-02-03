import * as path from 'path'
import { versions } from '../../../../.versions'
import { cleanDestinationDirectory, crd2pulumi } from '../../../../libs/gen'

const CERTMANAGER_URL = `https://github.com/cert-manager/cert-manager/releases/download/v${versions.certManager}/cert-manager.crds.yaml`

export async function certmanager() {
    const destination = path.join(__dirname, '../crds')
    await cleanDestinationDirectory(destination)
    await crd2pulumi({ destination, sources: [CERTMANAGER_URL] })
}
