# Cluster foundation

Ansible owns K3s on pi-0, pi-1, and nix-gpu-3070-1. pi-2 is excluded.
Flux owns kube-vip and Cilium after bootstrap. Both tools consume the definitions
in `clusters/homelab/`; change versions and settings there, never in a second
bootstrap configuration. Applications and storage are a later migration.

## Local prerequisites and checks

Use `nix develop` for Nix, uv, kubectl, Helm, Flux, and sshpass. Unlock git-crypt
so Ansible can read `infra.yaml` and the existing Nix server SSH key. The Python
and collection dependencies are pinned; the wrapper installs them locally.

Run `scripts/homelab check` to render shared definitions, template Cilium with
the actual release values, test upgrade guards, and syntax-check all playbooks.
Private kubeconfigs, SSH known-host records, and tooling live in `.homelab/`.
Do not print evaluated Ansible inventory: it includes SSH credentials.

## Initialization

Backups and cluster teardown are maintained separately. These playbooks require
clean hosts or an existing cluster already owned by this Ansible configuration;
they refuse to overwrite unmanaged cluster state.

1. Publish the shared cluster definitions to `main` before enabling Flux.
   Flux is enabled in the inventory after the initial foundation review.
   For a pre-publication bootstrap, override with `-e enable_flux=false`.
2. Run `scripts/homelab prepare-nixos` to build the NixOS prerequisite changes
   without switching. The pinned nixos-rebuild-ng tool builds on the Nix server.
   Run it with `-e apply_nixos_runtime=true` to apply the reviewed configuration.
   This removes NixOS K3s service ownership and installs native dependencies and
   the persistent Ansible unit generator.
3. Run `scripts/homelab bootstrap`. pi-0 starts first, then the other servers
   join through its direct address. Phase 2 installs kube-vip and Cilium from
   shared definitions, then installs Flux and hands reconciliation to Git.
   With `enable_flux=false`, Flux installation and checks are skipped.
   Verification checks the nodes, VIP API, DNS, and cross-node traffic.
4. Repeat `scripts/homelab bootstrap` to check convergence, then run
   `scripts/homelab failover -e run_disruptive_checks=true`. This stops the
   actual VIP owner, verifies failover, restores its API, then reboots
   `reboot_test_host` (pi-0 by default) to check service persistence.

`phase1` and `phase2` can be run separately to resume a failed bootstrap.
Initialization refuses configuration and identity conflicts. Matching K3s
inputs do not restart the service. Once Flux owns the add-ons, ordinary
phase 2 verifies them instead of reinstalling them.

## Flux handoff

The initial foundation PR has been merged and `enable_flux: true` is set in
`ansible/inventory/group_vars/all.yml`. Run `scripts/homelab phase2` and
`scripts/homelab verify` to install or verify the handoff. Shared cluster
manifests must match published main before installation. Flux adopts the same
Cilium release and kube-vip definitions. Keep the setting true for later runs.

## Upgrades

Change the exact K3s version in `ansible/inventory/group_vars/all.yml`. For the
Nix server also select a nixpkgs revision and package attribute that provide
that exact version. Run checks, then `scripts/homelab upgrade`. It stages and
checks the binary, drains one server, upgrades it, waits for readiness, and
uncordons before proceeding. Downgrades and skipped Kubernetes minors are
rejected. A failed host remains cordoned and stops the rollout; investigate
before resuming. Ordinary bootstrap refuses version changes.

For kube-vip, Cilium, or Flux, change their pinned shared definitions and publish
to main. Flux reconciles them. No second version declaration in Ansible exists.
Keep etcd snapshots and the server token for later recovery; automated
off-cluster snapshot backups are deferred.

## Recovery

While Flux is disabled, use `scripts/homelab phase2 -e allow_addon_recovery=true`
for deliberate add-on repair.

Run `scripts/homelab recover -e allow_addon_recovery=true` to suspend Flux's
foundation reconciliation and repair the same definitions through a healthy
direct-node API. Successful online repair resumes reconciliation. Cilium repair performs a Helm
upgrade even when release metadata matches, so missing resources are recreated.
With Helm 4, deliberate repair uses server-side conflict takeover while Flux
is suspended; it does not replace or uninstall Cilium resources.
Application reconciliations are left alone. Failure leaves foundation
reconciliation suspended for investigation; it never automatically uninstalls
the networking foundation.

If Git is unavailable, add `-e resume_gitops=false` to repair from the checkout
and leave Flux suspended. Reconcile the checkout with published main before
running online recovery to resume ownership. A surviving node's kubeconfig can
be used if another host is unreachable. If no direct API is healthy, first
repair K3s or etcd over SSH; add-on recovery cannot repair lost etcd quorum.

Run `scripts/homelab verify` for node readiness, control-plane labels, kube-vip,
Cilium, CoreDNS, VIP API, Flux handoff, and cross-node service/DNS checks. During
an intentionally suspended Git outage use `-e verify_gitops=false`.
