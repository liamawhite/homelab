#!/usr/bin/env python3
"""Own one nftables table; never flush Cilium/K3s rules or open on failure."""
import argparse
import ipaddress
import json
from pathlib import Path
import re
import socket
import subprocess
import time


def addresses(values, networks=False):
    result = []
    for value in values:
        item = ipaddress.ip_network(value, strict=True) if networks else ipaddress.ip_address(value)
        if item.version != 4 or (networks and item.prefixlen == 0):
            raise ValueError('Only explicit IPv4 destinations/sources are supported')
        result.append(str(item))
    return result


def validate(config):
    if not config['administrators']:
        raise ValueError('Set ADMIN_CIDRS_IPV4 before activating the host firewall')
    for key in ['administrators', 'nodes', 'dns', 'dhcp']:
        addresses(config[key], networks=key == 'administrators')
    for key in ['https_hosts', 'ntp_hosts']:
        for host in config[key]:
            if not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9.-]*', host):
                raise ValueError('Expected an explicit DNS hostname')
    addresses([config['vip']])
    addresses([config['pod_cidr']], networks=True)


def resolve(hosts, previous, now):
    """Retain last successful resolution for at most one hour on DNS failure."""
    records = {}
    for host in hosts:
        try:
            ips = sorted({a[4][0] for a in socket.getaddrinfo(host, None, socket.AF_INET)})
            if not ips:
                raise OSError('No IPv4 address')
            records[host] = {'addresses': addresses(ips), 'timestamp': now}
        except OSError:
            old = previous.get(host, {})
            if now - old.get('timestamp', 0) < 3600:
                records[host] = old
    return records


def render(config, cache, now):
    validate(config)
    def literal(items):
        return ', '.join(items)
    def elements(items):
        return 'elements = { ' + literal(items) + ' };' if items else ''
    def cached(hosts):
        return sorted({ip for host in hosts for ip in cache.get(host, {}).get('addresses', [])
                       if now - cache[host].get('timestamp', 0) < 3600})
    # INPUT/OUTPUT cover host processes and host-network exceptions. Pod forwarding
    # remains Cilium's responsibility; this table has no FORWARD accept bypass.
    return f'''add table inet homelab_host
delete table inet homelab_host
table inet homelab_host {{
  set admins {{ type ipv4_addr; flags interval; {elements(addresses(config['administrators'], True))} }}
  set nodes {{ type ipv4_addr; {elements(addresses(config['nodes'] + [config['vip']]))} }}
  set dns {{ type ipv4_addr; {elements(addresses(config['dns']))} }}
  set dhcp {{ type ipv4_addr; {elements(addresses(config['dhcp']))} }}
  set https {{ type ipv4_addr; {elements(addresses(cached(config['https_hosts'])))} }}
  set ntp {{ type ipv4_addr; {elements(addresses(cached(config['ntp_hosts'])))} }}
  chain input {{
    type filter hook input priority 10; policy drop;
    iifname "lo" accept
    ct state established,related accept
    ct state invalid drop
    ip saddr @admins tcp dport {{ 22, 6443 }} counter accept comment "administration and direct API recovery"
    ip saddr @nodes tcp dport {{ 2379, 2380, 6443, 10250, 4240 }} counter accept comment "etcd, API, kubelet, Cilium health"
    ip saddr @nodes icmp type echo-request counter accept comment "Cilium node health"
    ip saddr @nodes udp dport 8472 counter accept comment "Cilium VXLAN"
    ip saddr {config['pod_cidr']} tcp dport {{ 6443, 10250 }} counter accept comment "API clients and metrics-server; Cilium also authorizes source pods"
    ip saddr @dhcp udp sport 67 udp dport 68 counter accept comment "reserved DHCP leases"
    counter drop comment "unapproved host ingress"
  }}
  chain output {{
    type filter hook output priority 10; policy drop;
    oifname "lo" accept
    ct state established,related accept
    ct state invalid drop
    ip daddr @nodes tcp dport {{ 2379, 2380, 6443, 10250, 4240 }} counter accept comment "cluster bootstrap and control plane"
    ip daddr @nodes icmp type echo-request counter accept comment "Cilium node health"
    ip daddr @nodes udp dport 8472 counter accept comment "Cilium VXLAN"
    ip daddr {config['pod_cidr']} tcp dport {{ 4240, 8000, 8080, 8181, 9440, 10250, 15017, 15021 }} counter accept comment "declared foundation probes, aggregated API, and Istio admission webhook"
    ip daddr {config['pod_cidr']} icmp type echo-request counter accept comment "Cilium endpoint health"
    ip daddr @dns udp dport 53 counter accept comment "approved DNS resolver"
    ip daddr @dns tcp dport 53 counter accept
    ip daddr @ntp udp dport 123 counter accept comment "approved NTP hosts"
    ip daddr @https tcp dport 443 counter accept comment "approved image registries and bootstrap package hosts"
    ip daddr @dhcp udp sport 68 udp dport 67 counter accept
    ip daddr 255.255.255.255 udp sport 68 udp dport 67 counter accept comment "DHCP discovery"
    counter drop comment "unapproved host egress"
  }}
}}
'''


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--config', default='/etc/homelab/host-firewall.json')
    parser.add_argument('--cache', default='/var/lib/homelab/firewall-dns.json')
    parser.add_argument('--resolve', action='store_true')
    parser.add_argument('--print', action='store_true')
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    validate(config)
    cache_path = Path(args.cache)
    cache = json.loads(cache_path.read_text()) if cache_path.exists() else {}
    now = time.time()
    if args.resolve:
        cache = resolve(config['https_hosts'] + config['ntp_hosts'], cache, now)
    rules = render(config, cache, now)
    if args.print:
        print(rules)
        return
    subprocess.run(['nft', '--check', '--file', '-'], input=rules, text=True, check=True)
    # Atomic batch replacement: on validation/application failure existing rules stay.
    subprocess.run(['nft', '--file', '-'], input=rules, text=True, check=True)
    if args.resolve:
        temporary = cache_path.with_suffix('.tmp')
        temporary.write_text(json.dumps(cache))
        temporary.chmod(0o600)
        temporary.replace(cache_path)


if __name__ == '__main__':
    main()
