"""Check cold-bootstrap contracts and render the exact shared Helm values."""
from pathlib import Path
import subprocess
import tempfile
import yaml
from check_upgrade import validate

root = Path(__file__).resolve().parents[2]
cluster = root / 'clusters/homelab'

def run(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout

inventory = yaml.safe_load((root / 'ansible/inventory/hosts.yml').read_text())
for directory in ['kube-vip', 'cilium', 'flux-system', 'security', 'istio', 'mesh-security']:
    documents = list(yaml.safe_load_all(run('kubectl', 'kustomize', str(cluster / directory))))
    assert documents and all(d and d.get('apiVersion') and d.get('kind') for d in documents)

vip = yaml.safe_load((cluster / 'kube-vip/daemonset.yaml').read_text())['spec']['template']['spec']
assert vip['hostNetwork'] and vip['dnsPolicy'] == 'Default'
env = {e['name']: e for e in vip['containers'][0]['env']}
assert env['KUBERNETES_SERVICE_HOST']['valueFrom']['fieldRef']['fieldPath'] == 'status.hostIP'
assert {'operator': 'Exists'} in vip['tolerations']
release = yaml.safe_load((cluster / 'cilium/release.yaml').read_text())['spec']
assert release['releaseName'] == 'cilium'
assert release['storageNamespace'] == release['targetNamespace'] == 'kube-system'
repository = yaml.safe_load((cluster / 'cilium/repository.yaml').read_text())['spec']['url']
with tempfile.TemporaryDirectory() as tmp:
    values = Path(tmp) / 'values.yaml'
    values.write_text(yaml.safe_dump(release['values']))
    rendered = run('helm', 'template', release['releaseName'], release['chart']['spec']['chart'], '--repo', repository, '--version', release['chart']['spec']['version'], '--namespace', release['targetNamespace'], '-f', str(values))
    resources = list(yaml.safe_load_all(rendered))
    assert any(r and r['kind'] == 'DaemonSet' and r['metadata']['name'] == 'cilium' for r in resources)

for old, new in [('v1.36.2+k3s1', 'v1.36.2+k3s1'), ('v1.36.2+k3s1', 'v1.36.3+k3s1'), ('v1.36.2+k3s1', 'v1.37.0+k3s1')]:
    validate(old, new)
for old, new in [('v1.36.2+k3s1', 'v1.35.9+k3s1'), ('v1.36.2+k3s1', 'v1.38.0+k3s1'), ('v1.36.2+k3s1', 'latest')]:
    try:
        validate(old, new)
    except ValueError:
        pass
    else:
        raise AssertionError('Unsafe upgrade accepted')
# Exercise Ansible's native templating, since a syntactically valid expression
# can still return a string instead of Kubernetes objects during recovery.
role = yaml.safe_load((root / 'ansible/roles/addons/tasks/main.yml').read_text())
block = next(task for task in role if 'block' in task)['block']
flux_block = next(task for task in block if task['name'] == 'Hand off the verified foundation to Flux')
assert 'when' not in flux_block
apply = next(task for task in flux_block['block'] if task['name'].startswith('Apply the shared'))
settings = yaml.safe_load((root / 'ansible/inventory/group_vars/all.yml').read_text())
assert settings['verify_gitops'] is True
recovery_tasks = [
    {'ansible.builtin.set_fact': {'rendered': apply['kubernetes.core.k8s']['definition']}, 'loop': apply['loop'], 'register': 'renderings', 'no_log': True},
    {'ansible.builtin.assert': {'that': ["renderings.results[0].ansible_facts.rendered | type_debug == 'dict'", "not renderings.results[0].ansible_facts.rendered.spec.suspend | default(false)", 'renderings.results[2].ansible_facts.rendered.spec.suspend'], 'quiet': True}},
]
with tempfile.TemporaryDirectory() as tmp:
    playbook = Path(tmp) / 'recovery-render.yml'
    playbook.write_text(yaml.safe_dump([{'hosts': 'localhost', 'connection': 'local', 'gather_facts': False, 'vars': {'allow_addon_recovery': True}, 'tasks': recovery_tasks}]))
    run('ansible-playbook', str(playbook))
print('Shared manifests, actual Cilium Helm values, bootstrap routing, recovery templating, and upgrade guards verified')

# Security contracts prevent generated chart changes from silently widening access.
security = list(yaml.safe_load_all((cluster / 'security/network.yaml').read_text()))
baseline = next(d for d in security if d['metadata']['name'] == 'homelab-default-deny')
assert baseline['spec']['endpointSelector'] == {}
assert baseline['spec']['enableDefaultDeny'] == {'ingress': True, 'egress': True}
assert baseline['metadata']['annotations']['kustomize.toolkit.fluxcd.io/prune'] == 'disabled'
assert release['values']['extraConfig']['allow-localhost'] == 'policy'
flux_resources = list(yaml.safe_load_all(run('kubectl', 'kustomize', str(cluster / 'flux-system'))))
assert not any(d['kind'] == 'NetworkPolicy' for d in flux_resources)
for d in security:
    assert d['spec']['description']
    for direction in ['ingress', 'egress']:
        for grant in d['spec'].get(direction, []):
            assert grant, 'Empty additive grant opens all traffic'
            assert 'world' not in grant.get('toEntities', [])
            assert grant.get('toPorts') or grant.get('icmps'), 'Every grant needs explicit ports or ICMP types'
            assert any(k in grant for k in ['fromEntities','fromEndpoints','fromCIDR','toEntities','toEndpoints','toCIDR','toFQDNs']), 'Every grant needs explicit peers'

for d in yaml.safe_load_all((cluster / 'istio/releases.yaml').read_text()):
    if d['kind'] != 'HelmRelease':
        continue
    spec = d['spec']
    with tempfile.TemporaryDirectory() as tmp:
        values = Path(tmp) / 'values.yaml'
        values.write_text(yaml.safe_dump(spec['values']))
        resources = list(yaml.safe_load_all(run('helm', 'template', spec['releaseName'], spec['chart']['spec']['chart'], '--repo', 'https://istio-release.storage.googleapis.com/charts', '--version', spec['chart']['spec']['version'], '--namespace', spec['targetNamespace'], '-f', str(values))))
        assert resources
        if spec['releaseName'] == 'istio-cni':
            config = next(r['data'] for r in resources if r and r['kind'] == 'ConfigMap')
            assert config['AMBIENT_ENABLED'] == config['AMBIENT_DNS_CAPTURE'] == 'true'
        if spec['releaseName'] == 'ztunnel':
            assert any(r and r['kind'] == 'DaemonSet' for r in resources)
mesh = list(yaml.safe_load_all((cluster / 'mesh-security/baseline.yaml').read_text()))
assert any(d['kind'] == 'PeerAuthentication' and d['spec']['mtls']['mode'] == 'STRICT' for d in mesh)
assert any(d['kind'] == 'AuthorizationPolicy' and d['spec'] == {'action': 'ALLOW', 'rules': []} for d in mesh)
# Deliberately dormant egress: this change grants no application external service.
assert not any(d['kind'] == 'ServiceEntry' for d in mesh)
print('Pod deny coverage, explicit peer/port grants, localhost enforcement, Flux grant removal, ambient chart configuration, strict mTLS, and workload authorization baseline verified')
