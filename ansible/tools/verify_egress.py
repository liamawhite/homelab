"""Verify gateway TLS origination and denied bypass; retain failures for diagnosis."""
import argparse
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
NAMESPACE = 'flux-egress-verification'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--context', default='homelab')
    parser.add_argument('--disruptive', action='store_true', help='Briefly stop the egress waypoint, restoring it in finally')
    args = parser.parse_args()
    kubectl = ['kubectl', '--context=' + args.context]

    def run(*arguments):
        return subprocess.run(kubectl + list(arguments), capture_output=True, text=True, check=True).stdout

    def request(pod, url, expected=None):
        result = subprocess.run(kubectl + ['-n', NAMESPACE, 'exec', pod, '--', 'curl', '-sS', '--connect-timeout', '3', '--max-time', '8', '-o', '/dev/null', '-w', '%{http_code}', url], capture_output=True, text=True)
        if expected is None:
            assert result.returncode != 0 and result.stdout == '000', (url, result.stdout, result.stderr)
        else:
            assert result.returncode == 0 and result.stdout == str(expected), (url, result.stdout, result.stderr)

    workloads = str(ROOT / 'diagnostics/flux-egress/workloads.yaml')
    run('apply', '-f', workloads)
    run('-n', NAMESPACE, 'wait', '--for=condition=Ready', 'pod/client', 'pod/intruder', '--timeout=60s')
    for url in ['http://github.com/liamawhite/homelab.git/info/refs?service=git-upload-pack', 'http://helm.cilium.io/index.yaml', 'http://istio-release.storage.googleapis.com/charts/base-1.30.2.tgz']:
        request('client', url, 200)
    request('intruder', 'http://helm.cilium.io/index.yaml', 403)
    request('client', 'http://github.com/', 403)
    for url in ['http://example.com/', 'http://1.1.1.1/', 'https://1.1.1.1/', 'https://github.com/']:
        request('client', url)

    # Inspect actual proxy configuration, not just the intended YAML.
    clusters = json.loads(subprocess.run(['istioctl', '--context=' + args.context, 'proxy-config', 'clusters', 'deployment/egress', '-n', 'istio-egress', '-o', 'json'], capture_output=True, text=True, check=True).stdout)
    for host in ['github.com', 'helm.cilium.io', 'istio-release.storage.googleapis.com']:
        cluster = next(c for c in clusters if c['name'] == 'inbound-vip|80|http|' + host)
        assert cluster['loadAssignment']['endpoints'][0]['lbEndpoints'][0]['endpoint']['address']['socketAddress']['portValue'] == 443
        socket = cluster['transportSocket']
        while socket['name'] != 'envoy.transport_sockets.tls':
            socket = socket['typedConfig']['transportSocket']
        tls = socket['typedConfig']
        assert tls['sni'] == host
        validation = tls['commonTlsContext']['combinedValidationContext']
        assert validation['validationContextSdsSecretConfig']['name'] == 'file-root:system'
        assert validation['defaultValidationContext']['matchSubjectAltNames'] == [{'exact': host}]
    print('Registered downloads, identities, paths, external TLS/certificate validation, and denied direct 80/443/HTTPS bypass verified')

    if args.disruptive:
        try:
            run('-n', 'istio-egress', 'scale', 'deployment/egress', '--replicas=0')
            run('-n', 'istio-egress', 'wait', '--for=delete', 'pod', '-l', 'gateway.networking.k8s.io/gateway-name=egress', '--timeout=30s')
            request('client', 'http://helm.cilium.io/index.yaml')
        finally:
            run('-n', 'istio-egress', 'scale', 'deployment/egress', '--replicas=1')
            run('-n', 'istio-egress', 'rollout', 'status', 'deployment/egress', '--timeout=60s')
        print('Gateway outage failed closed and waypoint recovered')

    # Failures above deliberately retain resources; successful verification cleans up.
    run('delete', '--ignore-not-found', '--wait=false', '-f', workloads)
    print('Temporary verification resources removed')


if __name__ == '__main__':
    main()
