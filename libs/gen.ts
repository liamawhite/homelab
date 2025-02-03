import axios from "axios";
import { execSync } from "child_process";
import * as fs from "fs/promises";
import * as os from "os";
import * as path from "path";

export async function createTmpdir(suffix: string) {
    const tmpdir = os.tmpdir()
    const dir = path.join(tmpdir, suffix)
    await fs.mkdir(dir, { recursive: true })
    return dir
}

export async function cleanDestinationDirectory(directory: string) {
    await fs.rm(directory, { recursive: true, force: true })
    await fs.mkdir(directory, { recursive: true })
}

export async function downloadToDirectory(url: string, directory: string) {
    const filename = path.basename(new URL(url).pathname)
    const filePath = path.join(directory, filename)

    const response = await axios.get(url, { responseType: 'arraybuffer' })
    const fileData = Buffer.from(response.data, 'binary')
    await fs.writeFile(filePath, fileData)
}

export async function downloadToFile(url: string, filePath: string) {
    await fs.mkdir(path.dirname(filePath), { recursive: true })
    const response = await axios.get(url, { responseType: 'arraybuffer' })
    const fileData = Buffer.from(response.data, 'binary')
    await fs.writeFile(filePath, fileData)
}

export async function crd2pulumi(args: {
    destination: string,
    sources: string[],
}) {
    const { destination, sources } = args
    execSync(`crd2pulumi --nodejsPath ${destination} --force ${sources.join(' ')}`)

    // Remove generated files we don't need
    await fs.rm(path.join(destination, 'tsconfig.json'))
    await fs.rm(path.join(destination, 'README.md'))
    await fs.rm(path.join(destination, 'package.json'))
}
