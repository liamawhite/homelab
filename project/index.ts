import * as fs from "fs";
import * as path from "path";
import { configureNetwork } from "./network";
import { configureCluster } from "./cluster";
import { loadConfig } from "./config";
import { configureVpn } from "./vpn";

const cfg = loadConfig()

const vpn = configureVpn(cfg)
const cluster = configureCluster(cfg)
const network = configureNetwork({ ...cfg, cluster })

// Write the kubeconfig to a file at repo root so we can use it easily
// This is gitignored so it won't be checked in
cluster.kubeconfig.apply(cfg => {
    fs.writeFileSync(path.join(__dirname, '../', 'kubeconfig'), cfg)
})


