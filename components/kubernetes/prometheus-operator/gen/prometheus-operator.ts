import * as path from 'path'
import { versions } from '../../../../.versions'
import { cleanDestinationDirectory, crd2pulumi, downloadToFile, filterCRDsFromYaml } from '../../../../libs/gen'

const PROMETHEUS_OPERATOR_BUNDLE_URL = `https://github.com/prometheus-operator/prometheus-operator/releases/download/v${versions.prometheusOperator}/bundle.yaml`

export async function prometheusOperator() {
    const crdsDestination = path.join(__dirname, '../crds')
    const crdYamlFile = path.join(__dirname, '../crds.yaml')
    const tempBundleFile = path.join(__dirname, '../bundle-temp.yaml')

    await cleanDestinationDirectory(crdsDestination)
    
    // Download the bundle.yaml
    await downloadToFile(PROMETHEUS_OPERATOR_BUNDLE_URL, tempBundleFile)
    
    // Filter out only CRDs and save to crds.yaml
    await filterCRDsFromYaml(tempBundleFile, crdYamlFile)
    
    // Generate TypeScript bindings
    await crd2pulumi({ destination: crdsDestination, sources: [crdYamlFile] })
    
    // Clean up temp file
    const fs = await import('fs/promises')
    await fs.rm(tempBundleFile, { force: true })
}