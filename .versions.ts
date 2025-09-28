import { execSync } from 'child_process'

// Get current git commit SHA synchronously
const currentCommitSha = execSync('git rev-parse HEAD', { encoding: 'utf8' }).trim()

export const versions = {
    certManager: '1.18.2',
    coredns: currentCommitSha,
    externalDns: '0.15.1',
    gatewayApi: '1.2.0',
    grafana: '11.6.2',
    istio: '1.26.3',
    k8sGateway: '3.2.3', // https://github.com/k8s-gateway/k8s_gateway/pkgs/container/charts%2Fk8s-gateway/versions
    kubeStateMetrics: '2.14.0',
    longhorn: '1.9.1',
    metallb: '0.15.2',
    nodeExporter: '1.9.1',
    pihole: '2024.07.0',
    prometheus: '3.5.0',
    prometheusOperator: '0.84.1',
    syncthing: '1.30.0',
    tailscale: '1.86.2',
    homeassistant: currentCommitSha,
}
