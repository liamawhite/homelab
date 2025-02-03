import * as tailscale from "@pulumi/tailscale";
import { loadConfig } from "./config";

export function configureVpn({
    tailscale: { admin: { clientId, clientSecret } },
}: ReturnType<typeof loadConfig>) {
    const opts = {
        provider: new tailscale.Provider('tailscale', { oauthClientId: clientId, oauthClientSecret: clientSecret })
    }

    const acl = new tailscale.Acl('acl', {
        acl: JSON.stringify({
            acls: [
                // Allow all connections (default)
                { "action": "accept", "src": ["*"], "dst": ["*:*"] },
            ],
            tagOwners: {
                "tag:k8s-operator": [],
                "tag:k8s": ["tag:k8s-operator"],
            }
        }),
    }, opts)

    return {
        ready: [acl],
    }
}
