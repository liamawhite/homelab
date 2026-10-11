"""Refresh the homelab entry without changing unrelated kubeconfig entries."""
import argparse
from copy import deepcopy
import os
from pathlib import Path
import tempfile

import yaml


def merge(source: Path, destination: Path, server: str) -> bool:
    fresh = yaml.safe_load(source.read_text())
    context = next(item for item in fresh['contexts']
                   if item['name'] == fresh['current-context'])
    cluster = next(item for item in fresh['clusters']
                   if item['name'] == context['context']['cluster'])
    user = next(item for item in fresh['users']
                if item['name'] == context['context']['user'])
    replacements = deepcopy({'contexts': context, 'clusters': cluster, 'users': user})
    for item in replacements.values():
        item['name'] = 'homelab'
    replacements['clusters']['cluster']['server'] = server
    replacements['contexts']['context'].update(cluster='homelab', user='homelab')

    # Follow an existing symlink, and keep relative certificate paths valid by
    # writing beside the destination rather than flattening unrelated entries.
    destination = destination.expanduser().resolve()
    existing = yaml.safe_load(destination.read_text()) if destination.exists() else None
    result = deepcopy(existing) if existing else {'apiVersion': 'v1', 'kind': 'Config'}
    for key, replacement in replacements.items():
        result[key] = [item for item in result.get(key, []) or []
                       if item['name'] != 'homelab'] + [replacement]
    result['current-context'] = 'homelab'
    if result == existing:
        destination.chmod(0o600)
        return False

    destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    descriptor, temporary = tempfile.mkstemp(dir=destination.parent, prefix='.kubeconfig-')
    try:
        with os.fdopen(descriptor, 'w') as output:
            yaml.safe_dump(result, output, sort_keys=False)
        os.replace(temporary, destination)
    finally:
        Path(temporary).unlink(missing_ok=True)
    return True


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('server')
    args = parser.parse_args()
    try:
        changed = merge(args.source, Path.home() / '.kube/config', args.server)
    except (OSError, ValueError, KeyError, TypeError, StopIteration, yaml.YAMLError):
        # YAML parser diagnostics can include credential-bearing source lines.
        parser.exit(1, 'Could not update kubeconfig; check source structure and destination permissions.\n')
    print('updated' if changed else 'unchanged')
