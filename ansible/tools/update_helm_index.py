"""Refresh pinned Istio metadata and HTTP URLs consumed through mesh TLS origination."""
from pathlib import Path
import urllib.request
import yaml

root = Path(__file__).resolve().parents[2]
cluster = root / 'clusters/homelab'
with urllib.request.urlopen('https://istio-release.storage.googleapis.com/charts/index.yaml', timeout=30) as response:
    upstream = yaml.safe_load(response.read())
entries = {}
for release in yaml.safe_load_all((cluster / 'istio/releases.yaml').read_text()):
    if release['kind'] != 'HelmRelease':
        continue
    chart = release['spec']['chart']['spec']
    entry = next(e for e in upstream['entries'][chart['chart']] if e['version'] == chart['version'])
    assert all(url.startswith('https://istio-release.storage.googleapis.com/charts/') for url in entry['urls']), 'Review new download destinations before updating the index'
    entry['urls'] = [url.replace('https:', 'http:', 1) for url in entry['urls']]
    entries[chart['chart']] = [entry]
path = cluster / 'mesh-security/helm-index.yaml'
documents = list(yaml.safe_load_all(path.read_text()))
next(d for d in documents if d['kind'] == 'ConfigMap')['data']['index.yaml'] = yaml.safe_dump({'apiVersion': 'v1', 'entries': entries}, sort_keys=False)
path.write_text('# Curated pinned metadata; HTTP download URLs are encrypted by the egress waypoint.\n' + yaml.safe_dump_all(documents, sort_keys=False))
print('Pinned Istio index refreshed; review gateway path permissions when changing chart versions')
