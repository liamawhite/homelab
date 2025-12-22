import * as path from 'path'
import { versions } from '../../../../.versions'
import {
    cleanDestinationDirectory,
    crd2pulumi,
    createTmpdir,
    downloadToFile,
    helmTemplate,
    untarFile,
} from '../../../../libs/gen'

const LONGHORN_URL = `https://github.com/longhorn/longhorn/releases/download/v${versions.longhorn}/charts.tar.gz`

export async function longhorn() {
    const tmpdir = await createTmpdir('longhorn')
    const tmpfile = path.join(tmpdir, 'charts.tar.gz')
    const destination = path.join(__dirname, '../crds')

    await cleanDestinationDirectory(destination)
    await downloadToFile(LONGHORN_URL, tmpfile)
    untarFile(tmpfile)
    await helmTemplate(path.join(tmpdir, 'charts/longhorn'), path.join(tmpdir, 'crds.yaml'))
    await crd2pulumi({ destination, sources: [path.join(tmpdir, 'crds.yaml')] })
}
