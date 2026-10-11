# Cluster bootstrap and recovery runbook

Run commands from the repository root. Ansible owns K3s installation, configuration,
services, and upgrades. Flux owns kube-vip, Cilium, and its own controllers after
handoff. Both bootstrap and repair consume `clusters/homelab/`.

| Host | Address | OS | Role |
| --- | --- | --- | --- |
| pi-0 | 192.168.1.51 | Debian | Seed, control plane, etcd |
| pi-1 | 192.168.1.52 | Debian | Control plane, etcd |
| nix-gpu-3070-1 | 192.168.1.60 | NixOS | Control plane, etcd |

The API VIP is `192.168.1.50:6443`. pi-2 is excluded. Applications and storage are
outside this foundation procedure. Backups, destructive teardown, and etcd
snapshot restoration are maintained separately.

## Prepare the operator machine

1. Check out the intended revision. For normal bootstrap and online recovery,
   the committed `clusters/homelab/` tree must match published `main`.
2. Enter the development shell and unlock the checkout with your existing
   git-crypt key or GPG identity:

   ```sh
   nix develop
   git-crypt unlock
   scripts/homelab check
   ```

   The shell supplies Nix, uv, kubectl, Helm, Flux, and sshpass. The wrapper
   installs pinned Python dependencies and the Kubernetes Ansible collection.
   Checks render manifests and Helm values, validate upgrade guards and recovery
   templating, and syntax-check playbooks. They do not deploy to the cluster.
3. Confirm SSH access and sudo privileges on all three hosts. Pi credentials are
   read from unlocked `infra.yaml`; the Nix server uses user `liam` and the key
   `os/ssh/nix-gpu-3070-1`. Host addresses and OS details live in
   `ansible/inventory/hosts.yml`.

Private kubeconfigs and local tooling live in ignored `.homelab/`. The wrapper
uses a private SSH known-hosts file and accepts first-time host keys. Check host
identity before accepting a changed key. Do not print evaluated inventory,
credentials, server tokens, or kubeconfigs into logs or PRs.

## Bootstrap a cluster

### 1. Establish the starting state

All three machines must already have their OS installed, stable addresses, SSH,
and sudo access. This procedure does not install Raspberry Pi OS. Each server
must have at least 4 GiB RAM and a supported ARM64 or x86-64 architecture.

Start with clean hosts or a cluster already owned by these playbooks. Ansible
refuses existing unmanaged K3s database state. If replacing an old cluster,
complete the separately maintained backup and teardown procedure first.
Do not create an ownership marker to bypass that refusal.

### 2. Prepare the NixOS runtime

Build the prerequisite OS configuration, then apply it:

```sh
scripts/homelab prepare-nixos
scripts/homelab prepare-nixos -e apply_nixos_runtime=true
```

The first command builds on the Nix server without switching. The second builds
and switches it. This removes NixOS ownership of the K3s service and installs
native runtime dependencies and the generator for Ansible's persistent unit.
When migrating an existing NixOS-managed cluster, switching can stop its old
K3s service; schedule that migration as part of the teardown window.

### 3. Initialize and verify

For definitions already published to `main`:

```sh
scripts/homelab bootstrap
```

The command runs these stages in order:

1. Preflight all hosts before installing K3s.
2. Initialize pi-0, then join pi-1 and the Nix server one at a time through the
   seed's direct API address.
3. Apply kube-vip through a healthy direct API and verify access through the VIP.
4. Install Cilium using the shared Helm release identity, chart version, and values.
5. Install Flux, configure its public Git source, and wait for reconciliation.
6. Verify the exact three Ready control-plane/etcd nodes, kube-vip, Cilium,
   CoreDNS, VIP TLS/API readiness, and cross-node service traffic and DNS.

Ordinary pods and node readiness may remain incomplete between K3s startup and
Cilium installation. Complete phase 2 before treating that as a bootstrap failure.

Flux is part of every complete bootstrap. To work on K3s separately before
publishing add-on definitions, run `scripts/homelab phase1`. After publishing,
complete the handoff and verification:

```sh
scripts/homelab phase2
scripts/homelab verify
```

Phase 2 fetches remote main and refuses uncommitted or unpublished cluster
manifest changes before handoff. Flux adopts the same Cilium Helm release.
It reads public Git over HTTPS without a deploy key.

### 4. Confirm convergence and failover

```sh
scripts/homelab bootstrap
scripts/homelab failover -e run_disruptive_checks=true
```

A matching bootstrap should make no remote changes or restart K3s. Verification
creates and removes temporary workloads in `homelab-verification`, so changes
reported for those workloads are expected.

Failover deliberately stops K3s on the actual VIP owner, checks that VIP API
access survives, restores that service, and reboots pi-0 by default. It then
verifies the foundation again. Run it when a brief service interruption is
acceptable. The Nix server previously failed to return from a software reboot;
its cause is unresolved. Keep the reboot target at pi-0 until investigating
that issue with console access available.

### If bootstrap stops

Read the failed task and correct its cause, then resume the affected phase:

```sh
scripts/homelab phase1
scripts/homelab phase2
scripts/homelab verify
```

Phase 1 safely
converges matching managed hosts and can finish incomplete installation or joins.
It refuses implicit version, identity, credential, and configuration migrations.
Phase 2 can resume installation. Once Flux owns the add-ons, normal phase 2
checks the handoff rather than overwriting those resources; use recovery for repair.

## Diagnose an incident

Choose a direct kubeconfig for a surviving server. Bootstrap, phase 2, recovery,
and verification export these from hosts over SSH. Existing files remain useful
when another host is unreachable. For example:

```sh
export KUBECONFIG="$PWD/.homelab/pi-0.yaml"
kubectl --request-timeout=5s get --raw=/readyz
kubectl get nodes -o wide
kubectl -n kube-system get pods -o wide
kubectl -n flux-system get deployments,gitrepositories,kustomizations,helmreleases
```

If pi-0's API is unavailable, try `.homelab/pi-1.yaml` or
`.homelab/nix-gpu-3070-1.yaml`. Inspect the relevant controller or agent:

```sh
kubectl -n kube-system logs daemonset/kube-vip --tail=100
kubectl -n kube-system logs daemonset/cilium --tail=100
kubectl -n flux-system logs deployment/kustomize-controller --tail=100
kubectl -n flux-system logs deployment/helm-controller --tail=100
kubectl -n flux-system get events --sort-by=.lastTimestamp
```

| Finding | Next action |
| --- | --- |
| Direct API works; VIP fails | Run add-on recovery through the direct API. |
| API works; Cilium or Flux is unhealthy | Inspect the failure, correct definitions if necessary, then recover. |
| Only one node is down | Restore that host over SSH; keep the other two servers running. |
| No direct API is ready | Repair K3s/etcd first; add-on recovery requires a working API. |
| Git is unavailable | Use recovery with reconciliation left suspended. |

A three-member etcd cluster requires two functioning members for quorum. A node
being NotReady does not by itself establish whether etcd quorum is healthy;
check direct API readiness and K3s logs.

## Recover kube-vip, Cilium, or Flux

### Online repair

Use a reviewed checkout matching published main:

```sh
scripts/homelab recover -e allow_addon_recovery=true
```

The playbook:

1. Checks agreement between local cluster definitions and published main.
2. Exports direct-node kubeconfigs and selects a ready API, bypassing the VIP.
3. Suspends foundation Kustomizations (`flux-system`, `kube-vip`, `cilium`) and
   Cilium's HelmRelease if present.
4. Reapplies kube-vip and verifies the VIP, repairs the Cilium release, then
   reapplies Flux controllers and reconciliation definitions.
5. Resumes foundation reconciliation and checks GitOps health and networking.

Application reconciliations are not suspended by this procedure. Direct API
access does not depend on the VIP or pod networking for the operator's requests;
controllers and workloads still need their dependencies to function.

Repair uses the versions and values in the checkout. It does not automatically
choose a previous working release. For a bad Git change, revert or correct it,
publish that fix, and use the matching checkout for recovery.

Cilium repair runs a Helm upgrade even when release metadata matches, allowing
missing resources to be recreated. Helm 4 conflict takeover is enabled during
this explicit repair while Flux is suspended. It does not force replacement or
uninstall networking resources.

### Repair during a Git outage

```sh
scripts/homelab recover -e allow_addon_recovery=true -e resume_gitops=false
scripts/homelab verify -e verify_gitops=false
```

This skips the published-main check, repairs from the local checkout, and leaves
foundation reconciliation suspended. It is not a fully offline procedure:
Helm chart downloads and container images may still require network access.

After Git returns, align the checkout with the intended published definitions,
then rerun normal online recovery. That resumes reconciliation and verifies it.

### If repair fails

Foundation reconciliation remains suspended if failure occurs before the resume
steps. A failure after those steps may leave it resumed. Inspect the actual state:

```sh
kubectl -n flux-system get kustomizations \
  -o custom-columns=NAME:.metadata.name,SUSPENDED:.spec.suspend
kubectl -n flux-system get helmrelease cilium \
  -o custom-columns=NAME:.metadata.name,SUSPENDED:.spec.suspend
```

Correct the failed dependency or definition and rerun recovery. Do not uninstall
Cilium or reset K3s as a generic retry. Recovery reuses the existing Helm identity.

If Flux CRDs themselves are missing, recovery may stop while trying to inspect
its reconciliation objects. Through a healthy direct API, restore the shared
Flux installation first:

```sh
kubectl apply --server-side --field-manager=homelab-bootstrap --force-conflicts \
  -f clusters/homelab/flux-system/gotk-components.yaml
scripts/homelab recover -e allow_addon_recovery=true
```

Then confirm the source and foundation reconciliation objects have been restored.
This missing-CRD scenario has not been exercised live.

### Before the first Flux handoff

If an interrupted bootstrap has not created Flux's reconciliation objects,
resume `scripts/homelab phase2`, then run `scripts/homelab verify`. For deliberate
repair of partially installed add-ons, use
`scripts/homelab phase2 -e allow_addon_recovery=true`; it also installs Flux.
If reconciliation objects already exist, use `recover` so they are suspended
before intervention.

## Recover a K3s host or control-plane outage

For a host that is reachable over SSH, inspect its service and recent journal:

```sh
sudo systemctl status k3s --no-pager
sudo journalctl -u k3s -b --no-pager -n 150
sudo systemctl cat k3s
```

Ansible's selected binary is `/var/lib/homelab/k3s/current/bin/k3s`; configuration
is `/etc/rancher/k3s/config.yaml`. Debian units live in `/etc/systemd/system/`.
The NixOS unit lives in `/var/lib/homelab/systemd/` and is exposed by its generator.
Avoid sharing logs that contain credentials.

Correct the identified host problem, then start an inactive service:

```sh
sudo systemctl start k3s
```

A managed-host convergence run (`scripts/homelab phase1`) can restore matching
installation inputs, but requires all inventory hosts to pass preflight. It is
not a lost-node replacement or etcd disaster-recovery procedure. If the NixOS
unit is missing from systemd, confirm its runtime generator is installed before
attempting convergence.

With no ready direct API, restore failed services, connectivity, or hosts first.
If etcd quorum or its data cannot be restored that way, use the separately
maintained snapshot restoration procedure and preserved server token. These
playbooks do not automate snapshot restore or membership replacement. Do not
bootstrap a second cluster on surviving data or delete database directories.

After a direct API is ready, run add-on recovery if needed, then full verification.

## Declare recovery complete

```sh
scripts/homelab verify
scripts/homelab phase2
```

Expect all three inventory nodes Ready with control-plane/etcd labels; kube-vip
and Cilium ready on every server; CoreDNS ready; VIP TLS/API access working;
foundation Flux reconciliation Ready and resumed; and cross-node HTTP/DNS tests
passing. Normal phase 2 should report zero changes after handoff.

`verify` requires all three intended nodes, so it can fail while a degraded
cluster still serves traffic. During an intentional Git outage, use
`-e verify_gitops=false` and keep track of the suspended reconciliation state.
Verification creates and removes temporary test workloads.

## Upgrades

For K3s, update the exact version in `ansible/inventory/group_vars/all.yml` and
select a Nix package revision/attribute providing that same version. Run checks,
then `scripts/homelab upgrade`. It stages and validates the binary, drains one
server respecting disruption budgets, upgrades it, waits for readiness, and
uncordons before continuing. Downgrades and skipped Kubernetes minors are
rejected. A failed host remains cordoned and halts the rollout; investigate before
resuming. Ordinary bootstrap refuses version changes. A real version upgrade
has not yet been validated live.

For kube-vip, Cilium, or Flux, update the pinned shared definitions and publish
to main. Flux reconciles them. Keep etcd snapshots and the server token through
the separately maintained backup process; off-cluster automation is deferred.

## Verification history

The three-server bootstrap, repeated convergence, VIP failover, and pi-0 reboot
passed. Flux adoption, post-handoff repair with reconciliation resumed, cross-node
HTTP/DNS tests, and zero-change phase 2 convergence also passed. NixOS reboot
persistence, lost-quorum restoration, missing-CRD recovery, and a real K3s version
upgrade have not been validated. See `docs/deployment-rewrite.md` for the migration
record and the unresolved Nix server reboot issue.
