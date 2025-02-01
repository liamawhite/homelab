import * as k8s from "@pulumi/kubernetes";
import { K3s } from "../components/k3s";
import { RaspberryPi } from "../components/raspberrypi5";
import { loadConfig } from "./config";

export function configureCluster({
    clusterToken,
    nodes,
}: ReturnType<typeof loadConfig>) {
    const servers = [
        new RaspberryPi('rpi-0', { connection: nodes['rpi-0'] }),
    ]

    const cluster = new K3s('k3s', {
        servers,
        clusterToken: clusterToken,
        sans: ['kube.local', 'kubernetes.local', 'k8s.local', 'k3s.local'],
    })

    return {
        ready: [cluster],
        kubeconfig: cluster.kubeconfig,
        provider: new k8s.Provider('k8s', { kubeconfig: cluster.kubeconfig })
    }
}
