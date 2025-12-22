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

const METALLB_URL = `https://github.com/metallb/metallb/releases/download/metallb-chart-${versions.metallb}/metallb-${versions.metallb}.tgz`

export async function metallb() {
    const tmpdir = await createTmpdir('metallb')
    const tmpfile = path.join(tmpdir, 'metallb.tgz')
    const destination = path.join(__dirname, '../crds')

    await cleanDestinationDirectory(destination)
    await downloadToFile(METALLB_URL, tmpfile)
    untarFile(tmpfile)
    await helmTemplate(path.join(tmpdir, 'metallb/charts/crds'), path.join(tmpdir, 'crds.yaml'))
    await crd2pulumi({ destination, sources: [path.join(tmpdir, 'crds.yaml')] })
}
