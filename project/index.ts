import * as pulumi from '@pulumi/pulumi'
import * as fs from 'fs'
import * as path from 'path'
import { loadConfig } from './config'
import { configureNetwork } from './network'
import { configureCluster } from './cluster'
import { configureVpn } from './vpn'
import { configureDns } from './dns'
import { configurePki } from './pki'
import { configureStorage } from './storage'
import { configureSyncthing } from './syncthing'

const cfg = loadConfig()

configureVpn(cfg)
const cluster = configureCluster(cfg)
const pki = configurePki(cluster)
const storage = configureStorage({ cluster, pki })
const network = configureNetwork({ ...cfg, cluster })
const dns = configureDns({ ...cfg, cluster, network })
const syncthing = configureSyncthing({ cluster, pki, storage })

// Write the kubeconfig to a file at repo root so we can use it easily
// This is gitignored so it won't be checked in
cluster.kubeconfig.apply(cfg => fs.writeFileSync(path.join(__dirname, '../', 'kubeconfig'), cfg))

// Write the root CA cert to a file at repo root so we can easliy load it into machines
// This is gitignored so it won't be checked in but doesn't really matter if it is
pulumi
    .output(pki.ca.cert)
    .apply(pem => fs.writeFileSync(path.join(__dirname, '../', 'ca.pem'), pem))

export const passwords = {}

export const kubeconfig = cluster.kubeconfig
