import { execSync } from 'child_process'
import * as fs from 'fs/promises'
import * as path from 'path'
import * as os from 'os'
import * as yaml from 'js-yaml'

/**
 * Home Assistant Configuration Extractor
 * Extracts UI-managed configurations from .storage/ and converts them to YAML
 * for GitOps workflows.
 */

interface HAStorageData {
    data: any
    version: number
}

interface AutomationData extends HAStorageData {
    data: Array<{
        id: string
        alias: string
        trigger: any[]
        condition?: any[]
        action: any[]
        [key: string]: any
    }>
}

interface ScriptData extends HAStorageData {
    data: Record<
        string,
        {
            alias: string
            sequence: any[]
            [key: string]: any
        }
    >
}

async function loadJsonFile(filePath: string): Promise<any | null> {
    try {
        const content = await fs.readFile(filePath, 'utf8')
        return JSON.parse(content)
    } catch (error) {
        console.warn(`Warning: Could not load ${filePath}:`, error)
        return null
    }
}

async function copyFilesFromPod(): Promise<string | null> {
    const repoRoot = path.join(__dirname, '../../..')
    const kubeconfig = path.join(repoRoot, 'kubeconfig')

    try {
        await fs.access(kubeconfig)
    } catch {
        console.error(`Error: kubeconfig not found at ${kubeconfig}`)
        console.error("Run 'yarn deploy' first to generate the kubeconfig")
        return null
    }

    // Create temporary directory
    const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'ha-config-'))
    const storagePath = path.join(tempDir, '.storage')
    await fs.mkdir(storagePath, { recursive: true })

    console.log(`Using kubeconfig: ${kubeconfig}`)
    console.log(`Copying files to: ${tempDir}`)

    try {
        execSync(
            `kubectl --kubeconfig="${kubeconfig}" cp homeassistant/homeassistant-0:/config/.storage "${storagePath}"`,
            { stdio: 'inherit' },
        )
        console.log('Successfully copied .storage directory from Home Assistant pod')
        return tempDir
    } catch (error) {
        console.error('Error copying files from pod:', error)
        return null
    }
}

async function extractAutomations(storagePath: string, outputPath: string): Promise<boolean> {
    const automationsFile = path.join(storagePath, 'automations')

    try {
        await fs.access(automationsFile)
    } catch {
        console.log('No automations found')
        return false
    }

    const data = (await loadJsonFile(automationsFile)) as AutomationData
    if (!data?.data) return false

    // Convert to YAML format expected by HA
    const yamlAutomations = data.data.map(auto => {
        const { id, ...cleanAuto } = auto
        return cleanAuto
    })

    const outputFile = path.join(outputPath, 'automations.yaml')
    const yamlContent = yaml.dump(yamlAutomations, { lineWidth: -1, noRefs: true })
    await fs.writeFile(outputFile, yamlContent)

    console.log(`Extracted ${yamlAutomations.length} automations to ${outputFile}`)
    return true
}

async function extractScripts(storagePath: string, outputPath: string): Promise<boolean> {
    const scriptsFile = path.join(storagePath, 'scripts')

    try {
        await fs.access(scriptsFile)
    } catch {
        console.log('No scripts found')
        return false
    }

    const data = (await loadJsonFile(scriptsFile)) as ScriptData
    if (!data?.data) return false

    // Clean up scripts for YAML export
    const cleanScripts: Record<string, any> = {}
    for (const [scriptId, scriptData] of Object.entries(data.data)) {
        const { id, last_triggered, ...cleanScript } = scriptData as any
        cleanScripts[scriptId] = cleanScript
    }

    const outputFile = path.join(outputPath, 'scripts.yaml')
    const yamlContent = yaml.dump(cleanScripts, { lineWidth: -1, noRefs: true })
    await fs.writeFile(outputFile, yamlContent)

    console.log(`Extracted ${Object.keys(cleanScripts).length} scripts to ${outputFile}`)
    return true
}

async function extractScenes(storagePath: string, outputPath: string): Promise<boolean> {
    const scenesFile = path.join(storagePath, 'scenes')

    try {
        await fs.access(scenesFile)
    } catch {
        console.log('No scenes found')
        return false
    }

    const data = await loadJsonFile(scenesFile)
    if (!data?.data) return false

    // Convert scenes for YAML export
    const yamlScenes = data.data.map((scene: any) => {
        const { id, ...cleanScene } = scene
        return cleanScene
    })

    const outputFile = path.join(outputPath, 'scenes.yaml')
    const yamlContent = yaml.dump(yamlScenes, { lineWidth: -1, noRefs: true })
    await fs.writeFile(outputFile, yamlContent)

    console.log(`Extracted ${yamlScenes.length} scenes to ${outputFile}`)
    return true
}

async function extractLovelace(storagePath: string, outputPath: string): Promise<boolean> {
    const lovelaceFile = path.join(storagePath, 'lovelace')

    try {
        await fs.access(lovelaceFile)
    } catch {
        console.log('No Lovelace configuration found')
        return false
    }

    const data = await loadJsonFile(lovelaceFile)
    if (!data?.data) return false

    const outputFile = path.join(outputPath, 'ui-lovelace.yaml')
    const yamlContent = yaml.dump(data.data, { lineWidth: -1, noRefs: true })
    await fs.writeFile(outputFile, yamlContent)

    console.log(`Extracted Lovelace configuration to ${outputFile}`)
    return true
}

async function extractIntegrations(storagePath: string, outputPath: string): Promise<boolean> {
    const configEntriesFile = path.join(storagePath, 'core.config_entries')

    try {
        await fs.access(configEntriesFile)
    } catch {
        console.log('No integration configurations found')
        return false
    }

    const data = await loadJsonFile(configEntriesFile)
    if (!data?.data?.entries) return false

    const integrations: Record<string, any[]> = {}

    for (const entry of data.data.entries) {
        const domain = entry.domain || 'unknown'
        const title = entry.title || 'untitled'

        if (!integrations[domain]) {
            integrations[domain] = []
        }

        // Clean up sensitive data
        integrations[domain].push({
            title,
            options: entry.options || {},
            // Note: Excluding sensitive data like tokens
        })
    }

    const outputFile = path.join(outputPath, 'integrations.yaml')
    const header = `# Integration configurations extracted from UI
# Note: Sensitive data like tokens are not included
# This is for reference only - actual config may need manual setup

`
    const yamlContent = yaml.dump(integrations, { lineWidth: -1, noRefs: true })
    await fs.writeFile(outputFile, header + yamlContent)

    console.log(`Extracted ${Object.keys(integrations).length} integration types to ${outputFile}`)
    return true
}

async function generateMainConfig(outputPath: string, extractedFiles: string[]): Promise<void> {
    const configLines = [
        '# Home Assistant configuration',
        '# Generated by extract-config.ts',
        `# Extracted on: ${new Date().toISOString()}`,
        '',
        '# Core configuration',
        'default_config:',
        'frontend:',
        'config:',
        '',
        '# Network configuration',
        'http:',
        '  use_x_forwarded_for: true',
        '  trusted_proxies:',
        '    - 10.0.0.0/8',
        '    - 172.16.0.0/12',
        '    - 192.168.0.0/16',
        '',
        '# Database configuration',
        'recorder:',
        '  db_url: sqlite:////config/home-assistant_v2.db',
        '  purge_keep_days: 7',
        '',
        '# Logger configuration',
        'logger:',
        '  default: info',
        '',
        '# History configuration',
        'history:',
        '  include:',
        '    domains:',
        '      - sensor',
        '      - binary_sensor',
        '      - light',
        '      - switch',
        '',
    ]

    // Add includes for extracted files
    if (extractedFiles.includes('automations.yaml')) {
        configLines.push('# Automations (extracted from UI)')
        configLines.push('automation: !include automations.yaml')
        configLines.push('')
    }

    if (extractedFiles.includes('scripts.yaml')) {
        configLines.push('# Scripts (extracted from UI)')
        configLines.push('script: !include scripts.yaml')
        configLines.push('')
    }

    if (extractedFiles.includes('scenes.yaml')) {
        configLines.push('# Scenes (extracted from UI)')
        configLines.push('scene: !include scenes.yaml')
        configLines.push('')
    }

    const outputFile = path.join(outputPath, 'configuration.yaml')
    await fs.writeFile(outputFile, configLines.join('\n'))

    console.log(`Generated main configuration file: ${outputFile}`)
}

export async function extractHomeAssistantConfig(): Promise<number> {
    console.log('Home Assistant Configuration Extractor')
    console.log('='.repeat(50))

    // Copy files from pod
    const tempDir = await copyFilesFromPod()
    if (!tempDir) {
        return 1
    }

    const storagePath = path.join(tempDir, '.storage')
    const outputPath = path.join(__dirname, 'config')

    // Create output directory
    await fs.mkdir(outputPath, { recursive: true })

    console.log(`Extracting Home Assistant configurations...`)
    console.log(`From: ${storagePath}`)
    console.log(`To: ${outputPath}`)
    console.log('-'.repeat(50))

    const extractedFiles: string[] = []

    // Extract each type of configuration
    if (await extractAutomations(storagePath, outputPath)) {
        extractedFiles.push('automations.yaml')
    }

    if (await extractScripts(storagePath, outputPath)) {
        extractedFiles.push('scripts.yaml')
    }

    if (await extractScenes(storagePath, outputPath)) {
        extractedFiles.push('scenes.yaml')
    }

    if (await extractLovelace(storagePath, outputPath)) {
        extractedFiles.push('ui-lovelace.yaml')
    }

    if (await extractIntegrations(storagePath, outputPath)) {
        extractedFiles.push('integrations.yaml')
    }

    // Generate main config file
    await generateMainConfig(outputPath, extractedFiles)

    console.log('-'.repeat(50))
    console.log(`Extraction complete! ${extractedFiles.length} files extracted.`)
    console.log('\nNext steps:')
    console.log('1. Review the extracted YAML files in components/kubernetes/homeassistant/config/')
    console.log('2. Update your ConfigMap with the new configurations')
    console.log('3. Deploy the updated configuration')
    console.log('4. Verify Home Assistant loads correctly')

    // Clean up temp directory
    await fs.rm(tempDir, { recursive: true, force: true })

    return 0
}

// Allow direct execution
if (require.main === module) {
    extractHomeAssistantConfig()
        .then(code => process.exit(code))
        .catch(error => {
            console.error('Error:', error)
            process.exit(1)
        })
}
