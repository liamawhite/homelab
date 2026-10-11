"""Linux-only integration check in isolated network namespaces; requires root and nft.

Never loads rules into the host's network namespace. Removes only its own temporary
namespaces and files. Pass --firewall and --nft when testing from a staging directory.
"""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import uuid


def run(*args, **kwargs):
    result = subprocess.run(args, capture_output=True, text=True, **kwargs)
    if result.returncode:
        raise RuntimeError(result.stderr or result.stdout)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--firewall', default=str(Path(__file__).parents[1] / 'roles/host_firewall/files/host_firewall.py'))
    parser.add_argument('--nft', default='nft')
    args = parser.parse_args()
    python = shutil.which('python3')
    suffix = uuid.uuid4().hex[:8]
    host, peer = 'hl-host-'+suffix, 'hl-peer-'+suffix
    veth_host, veth_peer = 'hlh'+suffix, 'hlp'+suffix
    servers = []
    created = []
    def net(namespace, *command, **kwargs):
        return run('ip', 'netns', 'exec', namespace, *command, **kwargs)
    def connection(namespace, address, port, allowed, source=None):
        code = '''import socket,sys
s=socket.socket();s.settimeout(1)
if sys.argv[3] != '-': s.bind((sys.argv[3],0))
try:s.connect((sys.argv[1],int(sys.argv[2]))); connected=True
except (TimeoutError,OSError):connected=False
s.close()
assert connected == (sys.argv[4]=='yes'), (sys.argv[1:4],connected)
'''
        net(namespace, python, '-c', code, address, str(port), source or '-', 'yes' if allowed else 'no')
    try:
        for namespace in [host, peer]:
            run('ip','netns','add',namespace);created.append(namespace)
            run('ip','-n',namespace,'link','set','lo','up')
        run('ip','link','add',veth_host,'type','veth','peer','name',veth_peer)
        for namespace, interface, address in [(host,veth_host,'192.0.2.2/24'),(peer,veth_peer,'192.0.2.1/24')]:
            run('ip','link','set',interface,'netns',namespace)
            run('ip','-n',namespace,'addr','add',address,'dev',interface)
            run('ip','-n',namespace,'link','set',interface,'up')
        run('ip','-n',peer,'addr','add','192.0.2.3/24','dev',veth_peer)
        # Servers are listening before policy, so a timeout cannot be mistaken for
        # a closed port. The peer is the approved admin AND infrastructure HTTPS IP.
        server_code='''import socket,sys,threading,time
def serve(p):
 s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('0.0.0.0',p));s.listen()
 while True:
  c,_=s.accept();c.close()
for p in map(int,sys.argv[1:]):threading.Thread(target=serve,args=(p,),daemon=True).start()
time.sleep(120)
'''
        for namespace, ports in [(host,['22','6443','8080']),(peer,['443','8080'])]:
            servers.append(subprocess.Popen(['ip','netns','exec',namespace,python,'-c',server_code,*ports]))
        time.sleep(.5)
        connection(peer,'192.0.2.2',8080,True)
        connection(host,'192.0.2.1',8080,True)
        with tempfile.TemporaryDirectory() as directory:
            config=Path(directory)/'config.json';cache=Path(directory)/'cache.json'
            settings={'administrators':['192.0.2.1/32'],'nodes':['198.51.100.1'],'vip':'198.51.100.2','pod_cidr':'10.42.0.0/16','dns':['192.0.2.1'],'dhcp':['192.0.2.1'],'https_hosts':['approved.example'],'ntp_hosts':[]}
            config.write_text(json.dumps(settings));cache.write_text(json.dumps({'approved.example':{'addresses':['192.0.2.1'],'timestamp':time.time()}}))
            # Make nft available under its command name without touching system paths.
            bin_dir=Path(directory)/'bin';bin_dir.mkdir();(bin_dir/'nft').symlink_to(shutil.which(args.nft) or args.nft)
            import os
            environment={**os.environ,'PATH':str(bin_dir)+':'+os.environ['PATH']}
            net(host,args.nft,'add','table','inet','unrelated_test')
            net(host,python,args.firewall,'--config',str(config),'--cache',str(cache),env=environment)
            connection(peer,'192.0.2.2',22,True)
            connection(peer,'192.0.2.2',6443,True)
            connection(peer,'192.0.2.2',8080,False)
            connection(peer,'192.0.2.2',22,False,source='192.0.2.3')
            connection(host,'192.0.2.1',443,True)
            connection(host,'192.0.2.1',8080,False)
            connection(host,'127.0.0.1',8080,True)
            # A second application is idempotent and preserves unrelated tables.
            net(host,python,args.firewall,'--config',str(config),'--cache',str(cache),env=environment)
            net(host,args.nft,'list','table','inet','unrelated_test')
            settings['administrators']=[];config.write_text(json.dumps(settings))
            rejected=subprocess.run(['ip','netns','exec',host,python,args.firewall,'--config',str(config),'--cache',str(cache)],env=environment,capture_output=True)
            assert rejected.returncode != 0
            connection(peer,'192.0.2.2',22,True)
            connection(peer,'192.0.2.2',8080,False)
        print('Isolated nftables checks passed: admin access, ingress/egress denial, loopback, atomic reload, invalid-input refusal, unrelated-table preservation')
    finally:
        for server in servers:
            server.terminate();server.wait()
        for namespace in reversed(created):
            subprocess.run(['ip','netns','delete',namespace],check=False)


if __name__=='__main__':main()
