# os/

Declarative NixOS config for `nix-gpu-3070-1` - an ASUS ROG Strix desktop
(RTX 3070) joining the existing `pi-0`/`pi-1`/`pi-2` k3s cluster (see the
repo root `CLAUDE.md`) as an additional control-plane/server node. Unlike
the Pi nodes, this node isn't provisioned by `homelab bootstrap`/`homelab
k3s` - it's fully Nix-managed, so it's intentionally absent from
`infra.yaml`'s `nodes[]` (which requires SSH password auth; this node uses
SSH keys).

Static IP: `192.168.1.60`. SSH key: `os/ssh/nix-gpu-3070-1{,.pub}`
(git-crypt'd, like `infra.yaml` and `.pulumi-state/`).

## Status

Installed and joined the cluster as of 2026-08-01, **without** the NVIDIA
module - see "Adding NVIDIA back" below for why and what's left.

## GPU scope

`modules/nvidia.nix` + `modules/k3s.nix` make the host *capable* of GPU
workloads (driver + containerd runtime wiring), but don't schedule any -
that needs a cluster-side `RuntimeClass` + NVIDIA device-plugin
`DaemonSet`, which belongs in a `pkg/components/` addition (parallel to
`pkg/components/longhorn`), not in this OS config. That's a follow-up once
NVIDIA is actually enabled on the host (see below).

### Adding NVIDIA back

`hosts/nix-gpu-3070-1/default.nix` currently does **not** import
`../../modules/nvidia.nix` - building it (NVIDIA driver + full graphics
stack: mesa, ffmpeg, pipewire, gstreamer) crashed the RAM-backed live
installer twice in a row at the same step (`nvidia-x11` + initrd build),
with zero swap to cushion it there. The base system was installed without
it instead, specifically so it'd have real zram swap and no tmpfs
constraints before attempting that build. To add it back:

1. Uncomment the `../../modules/nvidia.nix` import in
   `hosts/nix-gpu-3070-1/default.nix`.
2. Apply via the Day-2 command below. If it still fails, it's a real
   incompatibility (not a resource issue) and needs separate debugging.

## One-time bootstrap

0. **Boot a NixOS live USB (physical step, one-time).** `nixos-anywhere`'s
   remote install works by kexec-ing the target from its *currently running*
   Linux kernel into the NixOS installer over SSH - it can't do that from
   Windows, which this machine currently runs. So:
   - Flash the [NixOS minimal ISO](https://nixos.org/download) to a USB
     stick (`balenaEtcher`, `Rufus`, or `dd` from another Linux/macOS box).
   - Boot the machine from it - you may need to disable Windows Fast
     Startup and/or Secure Boot in the BIOS first, and set the USB as the
     boot device (F8/F2/Del at POST on most ASUS boards).
   - At the live console: `sudo -i`, then `passwd` to set a temporary root
     password, then `ip a` to note the DHCP-assigned IP. That's the
     `<current-target-ip>` used below - everything from here on is remote
     over SSH from your workstation, no more physical access needed.

1. **SSH keypair** (already generated in this repo) - if you ever need to
   regenerate it:
   ```
   ssh-keygen -t ed25519 -f os/ssh/nix-gpu-3070-1 -C nix-gpu-3070-1 -N ""
   ```
   `.gitattributes` already git-crypts `os/ssh/**` - make sure that line
   exists *before* committing either key file.

2. **Fill in real disk device paths.** SSH into the live USB environment
   (`ssh root@<current-target-ip>`, the temporary password from step 0) and
   run:
   ```
   ls -la /dev/disk/by-id/
   ```
   Replace the 2 `REPLACE-ME-*` placeholders in
   `hosts/nix-gpu-3070-1/disko.nix` (1 nvme + 1 HDD) with the real by-id
   paths. **If either disk was used before**, also `wipefs --all --force`
   both the partition and the whole disk device first - disko's script
   checks `blkid | grep -q TYPE=` before formatting a partition, and skips
   `mkfs` entirely if it finds a leftover filesystem signature from a prior
   use, even after the partition table itself has been recreated. This bit
   us on the HDD (still had a stale FAT32 signature) and produced a silent
   "wrong fs type" mount failure with no indication mkfs was ever skipped.

3. **Confirm the network interface name** (`ip -brief link` in the same SSH
   session - pick the one that's `UP`/`LOWER_UP`) and update
   `networking.interfaces.eno1` in `hosts/nix-gpu-3070-1/default.nix` if
   it isn't actually `eno1`.

4. **Stage the k3s join token.** It's the existing `cluster.token` value
   from the repo root's `infra.yaml`:
   ```
   mkdir -p /tmp/nix-gpu-3070-1-extra-files/etc
   yq -r '.cluster.token' ../infra.yaml > /tmp/nix-gpu-3070-1-extra-files/etc/k3s-token
   ```
   (`nixos-anywhere --extra-files` copies this directory's contents onto
   the target's root at install time, outside the Nix store, so the token
   never ends up in plaintext in a store path.)

5. **Run nixos-anywhere** against the live USB environment's IP from step 0
   (it only becomes `192.168.1.60` after rebooting into this config):
   ```
   nix run github:nix-community/nixos-anywhere -- \
     --flake .#nix-gpu-3070-1 \
     --generate-hardware-config nixos-generate-config ./hosts/nix-gpu-3070-1/hardware-configuration.nix \
     --extra-files /tmp/nix-gpu-3070-1-extra-files \
     root@<current-target-ip>
   ```
   This wipes and repartitions every disk named in `disko.nix` - confirm the
   by-id paths from step 2 one more time before running it.

6. **Commit the generated `hardware-configuration.nix`** (nixos-anywhere
   overwrites the placeholder in this repo via the flag above).

7. Add `192.168.1.60` to `infra.yaml`'s `cluster.sans` list by hand for
   consistency with the other nodes (not required for this node to work,
   since `--tls-san=192.168.1.60` is already passed directly in
   `modules/k3s.nix` - just keeps the two sources of truth aligned).

## Known gotchas hit during the real install

- **Stale filesystem signatures on reused disks** - see step 2 above.
- **k3s refusing to start with "critical configuration value mismatch
  between servers"** - `pkg/k3s/installer.go`'s CLI flags aren't the whole
  story. The Pi cluster's actual `/etc/rancher/k3s/config.yaml` also sets
  `flannel-backend: "none"` and `disable-network-policy: true` (set
  out-of-band, presumably alongside the Cilium rollout), and k3s treats
  both as "critical" values checked against the cluster's bootstrap data
  on every join. `modules/k3s.nix` now passes `--flannel-backend=none
  --disable-network-policy` to match - if this error ever recurs, `ssh` to
  a Pi and diff its `/etc/rancher/k3s/config.yaml` against
  `modules/k3s.nix`'s `extraFlags`.
- **Node fell back to booting the USB stick again after a successful
  `nixos-install`** - the install log showed `bootctl`/`nixos-install`
  print "Not booted with EFI or running in a container, skipping EFI
  variable modifications", meaning the systemd-boot files were copied to
  the ESP but never registered in UEFI NVRAM. Fix was physically removing
  the USB stick and disabling CSM/Legacy boot in the BIOS so the firmware
  actually scans the ESP's fallback path (`\EFI\BOOT\BOOTX64.EFI`) instead
  of falling through to the next boot device. If this recurs, check
  whether a persistent `efibootmgr` NVRAM entry exists once booted, and
  add one manually if not.
- **NVIDIA module build crashing the live installer** - see "Adding
  NVIDIA back" above.

## Multi-disk nodes and Longhorn

`nix-gpu-3070-1` has two physical disks (nvme root + a second HDD mounted
at `/mnt/storage`, see `disko.nix`), and Longhorn only auto-registers a
single default disk per node (`/var/lib/longhorn/` on the root
filesystem) - it has no way to discover other mounted filesystems on its
own.

Set `homelab.longhorn.extraDisks = [ "/mnt/whatever" ];` in a host's
`default.nix` (module defined in `modules/longhorn.nix`) to register extra
disks. This sets the
`node.longhorn.io/create-default-disk`/`node.longhorn.io/default-disks-config`
label+annotation on the Kubernetes Node object via a systemd oneshot as
early as possible after `k3s.service` starts - Longhorn's documented,
intended mechanism for pre-configuring a node's disks (verified against
the v1.9.1 docs). Two things about it are easy to get wrong (both bit
`nix-gpu-3070-1` in turn, 2026-08-02):

1. **It's dead code unless a cluster-wide Setting is on.**
   `longhorn-manager`'s `KubernetesNodeController.syncDefaultDisks`
   (`controller/kubernetes_node_controller.go`, confirmed against the
   v1.9.1 tag) starts with `if !requireLabel { return nil }`, gated on the
   `create-default-disk-labeled-nodes` Setting - which the Helm chart
   defaults to `false`. Until it's on, the label and annotation are never
   even read; Longhorn silently falls back to auto-creating one default
   disk regardless of what's set on the Node object. `pkg/components/longhorn`
   turns this on via `defaultSettings.createDefaultDiskLabeledNodes: true`
   in the chart's `Values` - required cluster-wide, not per-node.
2. **The annotation is exclusive, not additive.** Once the Setting is on,
   `syncDefaultDisks` does `node.Spec.Disks = <disks from the annotation>`
   outright - it does *not* also create the usual default root disk. The
   annotation JSON built in `modules/longhorn.nix` therefore always
   includes an explicit `/var/lib/longhorn/` entry alongside
   `cfg.extraDisks`; a bare list of just the extra disks would leave the
   node with *only* those disks and no root-filesystem storage at all.

It also **only takes effect while the node's Longhorn `Node` object has no
disks yet** (`syncDefaultDisks`'s other early return: `if
len(node.Spec.Disks) != 0 { return nil }`) - a genuinely new node's first
join, or a `Node` object that's been deliberately deleted. Once a node has
any disk registered (even a wrong one from a config bug, like
`nix-gpu-3070-1` hit above), changing the label/annotation does nothing
until its `Node` object is empty again. To force that on an existing node
(this is how `nix-gpu-3070-1`'s disks were migrated off the old Pulumi
Patch approach - since retired from `pkg/components/longhorn` entirely):

```
kubectl delete nodes.longhorn.io -n longhorn-system <node>
# The manager pod's own watch doesn't reliably notice the CR disappearing
# and recreate it - force a fresh list/re-sync:
kubectl delete pod -n longhorn-system -l app=longhorn-manager --field-selector spec.nodeName=<node>
```

then poll `kubectl get nodes.longhorn.io -n longhorn-system <node> -o yaml`
until `spec.disks` reappears, and check `status.diskStatus.*.conditions`
for `Ready`/`Schedulable`. Only do this when the node's existing disks have
no live replica data (check `kubectl get replicas.longhorn.io -n
longhorn-system -o wide` first) - Longhorn re-registering a disk from
scratch doesn't preserve prior volume data, it's a fresh registration, not
a migration.

One more subtlety: leave `storageReserved: 0` in each disk entry in the
annotation JSON rather than trying to compute a real reserved-byte value in
Nix. `types.CreateDisksFromAnnotation` (same file/tag) treats `0` as "not
set" and recomputes it from the *live* disk's actual capacity times the
`storage-reserved-percentage-for-default-disk` Setting - the same
percentage-based reservation the normal auto-created default disk gets.
Nix can't know a physical disk's real capacity at eval time anyway, so
this is also the only value that's actually correct here.

### Adding a disk to an already-joined node

Updating `extraDisks` and re-running the systemd unit does **nothing** on a
node that already has disks registered - see the empty-`spec.disks`
precondition above. For that case, add the disk directly through the
Longhorn UI instead (Node -> Edit node and disks -> add disk, pointing at
the new mount path) - a normal, one-time Longhorn operation. Nothing in
Pulumi owns `spec.disks` for an already-registered node, so there's
nothing for this to conflict with.

Still update `extraDisks` in the host's `default.nix` alongside the UI
change, purely so a from-scratch reprovision of this node (or a new node
cloned from it) registers all disks correctly on first join - it just
won't retroactively apply to the already-live node. The delete-CR-and-
restart-manager trick above *would* pick it up from the annotation, but
that re-registers every disk on the node from scratch (existing ones
included), so it's real churn - not worth it just to add one disk when the
UI does it in one step with zero disruption to what's already there.

## Day-2 changes

`nixos-rebuild` isn't available locally on macOS, and this node is
x86_64-linux while a Mac coordinator is aarch64-darwin - build remotely on
the target itself rather than locally:

```
NIX_SSHOPTS="-i ssh/nix-gpu-3070-1 -o IdentitiesOnly=yes" \
  nix run nixpkgs#nixos-rebuild -- switch --flake .#nix-gpu-3070-1 \
  --target-host liam@192.168.1.60 --build-host liam@192.168.1.60 --use-remote-sudo
```

## Verification

- `ssh -i ssh/nix-gpu-3070-1 liam@192.168.1.60` succeeds.
- `systemctl status k3s` is active.
- From a machine with the cluster kubeconfig: `kubectl --context homelab get nodes` shows `nix-gpu-3070-1` as `Ready` alongside `pi-0/1/2`, with no version-skew warnings.
- Once NVIDIA is added back (see above): `nvidia-smi` on the host shows the
  RTX 3070 and driver loaded, and as an optional containerd GPU smoke test
  ahead of the cluster-side device-plugin work:
  ```
  ctr --address /run/k3s/containerd/containerd.sock run --rm --gpus 0 \
    docker.io/nvidia/cuda:12.4.1-base-ubuntu22.04 gputest nvidia-smi
  ```
