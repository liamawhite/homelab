import * as path from 'path'
import { versions } from '../../../../.versions'
import {
    cleanDestinationDirectory,
    crd2pulumi,
    createTmpdir,
    downloadToFile,
} from '../../../../libs/gen'

export const GATEWAY_API_URL = `https://github.com/kubernetes-sigs/gateway-api/releases/download/v${versions.gatewayApi}/standard-install.yaml`

export async function gatewayapi() {
    const tmpdir = await createTmpdir('gatewayapi')
    const tmpfile = path.join(tmpdir, 'gatewayapi.yaml')
    const destination = path.join(__dirname, '../crds/gatewayapi')

    await cleanDestinationDirectory(destination)
    await downloadToFile(GATEWAY_API_URL, tmpfile)
    await crd2pulumi({ destination, sources: [tmpfile] })
}
