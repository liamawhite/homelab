import * as pulumi from '@pulumi/pulumi'
import { remote } from '@pulumi/command/types/input'

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
    return {
        nodes: {
            rp0: cfg.requireObject<remote.ConnectionArgs>('rp0'),
            rp1: cfg.requireObject<remote.ConnectionArgs>('rp1'),
            rp2: cfg.requireObject<remote.ConnectionArgs>('rp2'),
        },
        tailscale: cfg.requireObject<Tailscale>('tailscale'),
    }
}
