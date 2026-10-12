# Separate Tailscale cluster reconciliation from tailnet administration

Keep the existing tailnet and its access rules while migrating the Tailscale operator to Flux; limit policy changes to required tag ownership rather than carrying forward the legacy whole-policy overwrite. Ansible's Tailscale responsibilities are limited to delivering secrets from git-crypt-protected configuration into Kubernetes Secrets referenced by Flux; it does not manage tailnet policy or deploy Tailscale resources. Cloudflare integration is excluded, and production Tailscale hostnames are retained as their applications migrate.

Check existing tag ownership and have the homelab owner apply any missing ownership in the Tailscale admin console; do not automate policy writes. Begin without Tailscale-specific outbound grants and use live denied-flow evidence to propose individual connections for approval instead of carrying forward the legacy broad transport allowance.
