import { simpleGit } from 'simple-git'

// Get current git commit SHA
const git = simpleGit()
const currentCommitSha = await git.revparse(['HEAD'])

export const versions = {
    certManager: '1.16.3',
    coredns: currentCommitSha,
    externalDns: '0.15.1',
    gatewayApi: '1.2.0',
    istio: '1.26.3',
    k8sGateway: '3.2.3', // https://github.com/k8s-gateway/k8s_gateway/pkgs/container/charts%2Fk8s-gateway/versions
    longhorn: '1.9.1',
    metallb: '0.14.9',
    pihole: '2024.07.0',
    syncthing: '1.30.0',
    tailscale: '1.78.3',
}
