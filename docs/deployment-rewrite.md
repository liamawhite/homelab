# Deployment rewrite

## Settled requirements

- Prioritize simplicity and reproducible rebuilds.
- Retain K3s as the Kubernetes distribution.
- Use Ansible playbooks for cluster initialization and upgrades.
- Rebuild using the currently running K3s, Cilium, and kube-vip versions. Pin an initial Flux release. Validate the new deployment before testing separate, explicit upgrades.
- Defer off-cluster etcd backups for the foundation milestone; retain local K3s snapshots. Revisit backup requirements when introducing applications and persistent storage.
- Retain Cilium as the cluster networking implementation, with pinned versions and explicit Pi-compatible settings.
- Phase 1: Ansible initializes K3s. Phase 2: Ansible bootstraps kube-vip, Cilium, and Flux using shared checked-in definitions, then hands normal reconciliation to Flux. Until Cilium is installed, do not expect ordinary pods, cluster DNS, or all nodes to be Ready; the complete first milestone spans both phases.
- Keep one source of truth for kube-vip, Cilium, and Flux versions and configuration. Bootstrap and recovery consume the same definitions as Flux. After handoff, ordinary Ansible runs verify add-ons rather than competing with Flux; explicit recovery may intervene through direct node APIs.
- Flux watches this repository's `main` branch, scoped to a dedicated cluster directory. Application source and OS configuration remain outside Flux reconciliation scope.
- The repository is public. Flux reads it over anonymous HTTPS; no Git deploy key or personal access token is required. Use `clusters/homelab/` as its reconciliation directory.
- Keep git-crypt for Ansible bootstrap secrets. Ansible reads them from an unlocked local checkout; Flux consumes public cluster definitions without receiving the git-crypt key. Application secret management is deferred to the application phase.
- Preserve `applications/` and the broader OS setup. The user explicitly allows removing the NixOS-managed K3s service and associated integration changes so Ansible owns K3s on all hosts. Nix-driven Raspberry Pi OS installation remains a later milestone.
- Ansible owns K3s installation, configuration, service management, and upgrades on all three hosts. Validate NixOS runtime prerequisites and persistence across OS rebuilds before applying the ownership change.
- Preserve SQL database dumps rather than full-volume backups. The user excludes orphan replicas and permits discarding remaining storage during the eventual rebuild, after verifying the required database backups.
- Use a clean cluster rebuild with planned downtime, after securing any persistent data that must survive.
- First milestone: intended nodes Ready, working pod networking and DNS, API access through kube-vip, successful node reboot, and demonstrated control-plane failover. Storage, ingress, and application deployment follow later.
- Provide one convergence command, with separate diagnostics and recovery commands. Repeated convergence must safely recognize existing configuration.
- Active inventory: `pi-0`, `pi-1`, and `nix-gpu-3070-1`, all control-plane/etcd servers. Exclude `pi-2`, which refuses SSH and is absent from the live cluster.

## Proposed implementation contract

- Phase 1 initializes pi-0 as the seed and joins pi-1 and the Nix server through its direct API address. Pin K3s `v1.36.2+k3s1`; disable Flannel, kube-proxy, network policy, Traefik, and ServiceLB consistently.
- Phase 2 bootstraps kube-vip `v1.2.1` through a direct API endpoint, then Cilium `1.19.5`, then Flux. kube-vip must reach the API without depending on either the VIP or Cilium service routing. Verify this bootstrap behavior explicitly.
- Keep kube-vip manifests, Cilium release identity/chart/values, and Flux installation/sync manifests under `clusters/homelab/`. Ansible consumes these definitions; Flux subsequently owns reconciliation. Do not introduce K3s add-on controllers that compete for the same resources.
- K3s normal convergence leaves matching nodes untouched and finishes incomplete installation/join safely. Identity conflicts or changes requiring reset stop with diagnostics. Upgrades are explicit rolling operations with health gates; reset is a separate deliberate operation.
- Recovery uses SSH and direct-node API access plus the same checked-in add-on definitions. Suspend affected Flux reconciliation during intervention and restore agreement with Git afterward. No blanket automatic uninstall on failure for critical add-ons.
- Remove NixOS K3s service ownership while preserving required native runtime dependencies and adjusting existing K3s-coupled helpers. Validate persistence across NixOS rebuilds.
- Verify repeated convergence, all three Ready nodes, pod connectivity and DNS, VIP API access, one-node reboot, and VIP/control-plane failover. Failure stops progression.
- Implementation and local validation come before any cluster reset. Final deployment review must include verified backup availability and refreshing SQL dumps immediately before data disposal, since applications may have changed since the earlier dumps.

## Implementation checks

- Exact shared-definition format and adoption verification.
- NixOS runtime prerequisites and migration from its existing K3s service to Ansible ownership.
- Safe rebuild procedure and fresh verified SQL dumps at deployment time.
- Initial Flux release compatibility, cold bootstrap ordering, recovery, and acceptance verification.

These remaining entries concern implementation validation, not a change to the settled architecture. The user confirmed the shared understanding and authorized implementation.

The foundation has been rebuilt and verified on all three active servers. Shared manifests and exact Cilium Helm values render, and all playbooks pass local checks. The NixOS prerequisite configuration has been built and applied. Flux remains disabled pending PR review and merge. The dated sections below record the migration history, not the current cluster state.

## Read-only inspection: 2026-10-10

- Active inventory confirmed: `pi-0` (.51), `pi-1` (.52), and `nix-gpu-3070-1` (.60). All three are Ready control-plane/etcd nodes running K3s `v1.36.2+k3s1`; services are active, with ample memory/disk and no failed systemd units.
- `pi-2` (.53) refuses SSH and is absent from the cluster.
- Cilium and kube-vip each have three Ready DaemonSet pods. VIP .50 is on pi-1; API readiness succeeds. Failover has not been tested.
- Shopping, Parental, and Prometheus have healthy attached Longhorn volumes. Legacy retained volumes exist; recoverability is uncertain. No Longhorn backup records or last-backup values were found; backups remain unverified.
- Workouts is Pending because its `workouts-data` PVC is missing.
- A refreshed read-only storage audit found orphan Longhorn replica directories on both Pis for Workouts, rescue volumes, and two unidentified volume IDs absent from live PV/Longhorn inventory. One unidentified replica occupies roughly 20–23 GiB per Pi. Recoverability is not verified; the user subsequently excluded these from preservation.
- Grafana and Alertmanager use ephemeral volumes. No application hostPath, NFS, or local volumes were found in the inspected workloads. Local Longhorn snapshots exist, but no off-cluster Longhorn backup target or backup records were found.
- After considering custom orchestration and continuous host reconciliation, the user chose Ansible playbooks for initialization and upgrades.

## Database backups

Shopping, Parental, and Grafana SQL dumps were written to `/Users/liam/homelab-backups/20261011T044208Z-sql` using SQLite online backups of the live databases. Each dump restored successfully into a separate in-memory database, passed integrity checks, and matched the source snapshot's table row counts. A private manifest records source paths, sizes, and SHA-256 hashes. No source data or volumes were deleted.

The user explicitly chose to discard Reminders, Trips, and Finances on detached legacy volumes. Workouts and other orphan replicas are also excluded. Preserve the verified Shopping, Parental, and Grafana SQL dumps; remaining storage was authorized for disposal during the rebuild.

The reusable backup command was also verified against the live databases, producing fresh restored SQL dumps under `/Users/liam/homelab-backups/20261011T052559Z-sql`. A final refresh was taken immediately before downtime, as recorded below.

## Pre-Flux verification attempt: 2026-10-10

The user requested verification before publishing: Flux installation and handoff
checks are disabled with `enable_flux: false`; the enabling line is commented
beside it. Publication checks now run only when enabling Flux. Local manifest,
Helm, recovery-template, and playbook syntax checks passed.

Fresh live Shopping, Parental, and Grafana snapshots were captured and restored
under `/Users/liam/homelab-backups/20261011T054228Z-sql` before downtime. The NixOS
handoff configuration was switched successfully after adapting its existing
Longhorn helper to start with the Ansible-managed K3s service. Reboot handling
was corrected to include NixOS's native shutdown binary search path.

Both Pis completed the reset reboot with K3s disabled. The Nix server did not
return over SSH within the reboot timeout. The reset stopped at that error,
before any cluster or storage deletion. Existing state remains on all three
hosts; the old cluster services are stopped. Three-node live verification and
PR publication remain pending recovery of the Nix server at 192.168.1.60.

Backup and teardown commands were removed from this repository at the user’s request on 2026-10-11. They are maintained separately; existing private SQL dumps remain untouched. Ansible here covers initialization, upgrades, verification, and add-on recovery.

## Completed foundation verification: 2026-10-11

After the user manually rebooted the Nix server, SSH access returned. Previous-boot
logs showed shutdown reaching the final reboot sequence but did not establish
why it failed to return. Cleanup then completed and Ansible initialized a fresh
three-server cluster. Applications and storage have not been redeployed.

- All three control-plane/etcd nodes are Ready on K3s `v1.36.2+k3s1`.
- kube-vip `v1.2.1` and Cilium `1.19.5` each have three ready agents.
- VIP API access, CoreDNS, and cross-node HTTP/service/DNS tests passed.
- Repeated bootstrap changed no remote hosts; K3s process IDs and start times
  remained unchanged. Temporary verification workloads were created and removed.
- Stopping K3s on the actual VIP owner (the Nix server) demonstrated VIP failover.
  The service was restored, pi-0 rebooted successfully, and verification passed.
- The earlier Nix server reboot hang remains unexplained. NixOS boot persistence
  of the new Ansible-owned service has not yet been demonstrated by a reboot.
- Explicit pre-Flux add-on repair succeeded using the shared definitions.
- Flux `v2.9.6` definitions are checked in but installation remains disabled.
  Flux adoption and a real K3s version upgrade remain later validation steps.

## Flux handoff: 2026-10-11

PR #17 was merged and Flux installation enabled. All four controllers became
Ready, the public Git source fetched merged main, and all foundation
Kustomizations became Ready. Flux adopted the existing Cilium release under
the same identity and chart version.

Post-handoff recovery initially encountered Helm 4 server-side field conflicts
with helm-controller. The repair command now detects conflict-takeover support
and uses it during explicit recovery while Flux is suspended. Resources are
updated without replacement or uninstall. The retry repaired Cilium, resumed
Flux, and passed node readiness, VIP, DNS, and cross-node traffic checks.

Application and storage deployment remain the next phase. The earlier Nix
server reboot issue and real K3s version-upgrade validation remain outstanding.

After live handoff, the temporary Flux enablement switch was removed. Flux is
now included in every complete bootstrap; `phase1` still permits K3s-only work.
`resume_gitops=false` remains available for recovery during a Git outage, and
`verify_gitops=false` skips reconciliation checks while intentionally suspended.
