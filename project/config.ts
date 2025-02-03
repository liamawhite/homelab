import * as pulumi from '@pulumi/pulumi';
import { remote } from '@pulumi/command/types/input';

interface K3s {
    clusterToken: pulumi.Input<string>
}

interface Tailscale {
    admin: TailscaleCredentials
    operator: TailscaleCredentials
}

interface TailscaleCredentials {
    clientId: pulumi.Input<string>
    clientSecret: pulumi.Input<string>
}

export function loadConfig() {
    const cfg = new pulumi.Config()
    const k3s = cfg.requireObject<K3s>('k3s')
    return {
        clusterToken: k3s.clusterToken,
        nodes: {
            'rp0': cfg.requireObject<remote.ConnectionArgs>('rp0'),
        },
        tailscale: cfg.requireObject<Tailscale>('tailscale'),
    }
}
