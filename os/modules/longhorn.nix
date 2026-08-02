# Longhorn (pkg/components/longhorn) requires a running iscsid + iscsiadm
# on every node it schedules onto for RWO block volumes. Raspberry Pi
# nodes get this via `apt-get install open-iscsi nfs-common` in
# pkg/raspberry's bootstrap step; this mirrors that for a NixOS node, which
# doesn't go through that provisioner. Without it, longhorn-manager
# crash-loops with "failed to check environment, please make sure you have
# iscsiadm/open-iscsi".
{ config, lib, pkgs, ... }:

let
  cfg = config.homelab.longhorn;

  # In "config" mode Longhorn's KubernetesNodeController.syncDefaultDisks
  # (controller/kubernetes_node_controller.go) sets node.Spec.Disks =
  # <disks from this annotation> outright - it does NOT additionally create
  # its usual single default root-filesystem disk the way label value
  # "true" does. Omitting the root disk here means the node registers with
  # cfg.extraDisks and *nothing else* - confirmed live against nix-gpu-3070-1
  # 2026-08-02 (a fresh Node object came back with only /mnt/storage
  # registered, no /var/lib/longhorn/ disk at all). The root disk has to be
  # listed explicitly alongside the extras.
  disksConfigJson = builtins.toJSON ([{
    path = "/var/lib/longhorn/";
    allowScheduling = true;
    storageReserved = 0;
    tags = [ ];
  }] ++ map (path: {
    inherit path;
    allowScheduling = true;
    storageReserved = 0;
    tags = [ "extra" ];
  }) cfg.extraDisks);
in

{
  options.homelab.longhorn = {
    # Extra disk mount paths (beyond the root filesystem's default
    # /var/lib/longhorn) that Longhorn should register on THIS node.
    #
    # Longhorn only auto-creates a Node CR with a single default disk
    # (/var/lib/longhorn/ on root) when a node first joins - it has no
    # mechanism to discover other mounted filesystems on its own. The
    # "proper" declarative way to pre-configure extra disks is a
    # node.longhorn.io/create-default-disk=config LABEL plus a
    # node.longhorn.io/default-disks-config annotation (JSON) on the
    # Kubernetes Node object - verified against Longhorn v1.9.1 docs
    # (longhorn.io/docs/1.9.1/nodes-and-volumes/nodes/default-disk-and-node-config)
    # 2026-08-02. Critically, per those docs: "the configuration...only
    # takes effect when there are no existing disks or tags on the node" -
    # it's read ONCE at first registration, never retroactively. This
    # option only helps a node that hasn't joined the cluster yet.
    #
    # kubelet/k3s has a --node-label flag but no equivalent for
    # annotations (confirmed via `k3s server --help` on this node), so
    # there's no way to set the annotation atomically at registration the
    # way --node-label would. Both the label and annotation below are
    # instead set by a systemd oneshot as early after k3s starts as
    # possible - this narrows the race against Longhorn's own controller
    # claiming the node first, but doesn't eliminate it. If Longhorn wins
    # the race anyway, the fallback is the same Pulumi Patch approach
    # already used for nix-gpu-3070-1 (see pkg/components/longhorn).
    #
    # nix-gpu-3070-1 itself already had a Longhorn Node CR before this
    # module existed, so setting this for it now would do nothing - its
    # /mnt/storage disk is registered via that Pulumi Patch instead. This
    # option is for the NEXT multi-disk node.
    extraDisks = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      example = [ "/mnt/storage" ];
      description = "Extra disk mount paths for Longhorn to register on this node at first cluster join.";
    };
  };

  config = lib.mkMerge [
    {
      services.openiscsi = {
        enable = true;
        name = "iqn.2026-08.homelab:nix-gpu-3070-1";
      };

      environment.systemPackages = [ pkgs.nfs-utils ];
      boot.supportedFilesystems = [ "nfs" ];

      # longhorn-manager execs `iscsiadm` at the hardcoded FHS path
      # /usr/bin/iscsiadm inside the host's mount namespace (via nsenter) -
      # NixOS doesn't populate /usr/bin with Nix store binaries by default, so
      # even with services.openiscsi enabled above, that lookup fails with
      # "nsenter: failed to execute iscsiadm: No such file or directory". This
      # is the standard NixOS+Longhorn workaround: symlink it in explicitly.
      system.activationScripts.longhorn-iscsiadm-usr-bin = ''
        mkdir -p /usr/bin
        ln -sf ${pkgs.openiscsi}/bin/iscsiadm /usr/bin/iscsiadm
      '';
    }

    (lib.mkIf (cfg.extraDisks != [ ]) {
      systemd.services.longhorn-default-disks-config = {
        description = "Label/annotate this Kubernetes Node with Longhorn's default-disks-config, before Longhorn's own controller can auto-create a single-disk Node CR";
        after = [ "k3s.service" ];
        wants = [ "k3s.service" ];
        wantedBy = [ "multi-user.target" ];
        serviceConfig = {
          Type = "oneshot";
          # Not RemainAfterExit: harmless (idempotent --overwrite) to rerun
          # every boot, and a rerun is the only way this could ever recover
          # from losing the race against Longhorn's controller on a prior boot.
          Restart = "no";
        };
        path = [ pkgs.gnugrep pkgs.coreutils ];
        script = ''
          set -eu
          # k3s's own bundled kubectl subcommand - NixOS's k3s packaging
          # doesn't populate a /var/lib/rancher/k3s/bin/kubectl symlink the
          # way the upstream get.k3s.io installer does, so reference the
          # actual package binary directly (same package modules/k3s.nix
          # pins) rather than assuming a specific FHS path exists.
          KUBECTL="${pkgs.k3s_1_36}/bin/k3s kubectl --kubeconfig=/etc/rancher/k3s/k3s.yaml"
          # config.networking.hostName is known at build time - no need for
          # a runtime `hostname` call, which also isn't on PATH by default
          # here (NixOS doesn't include it in `coreutils`).
          NODE_NAME="${config.networking.hostName}"

          for i in $(seq 1 60); do
            if $KUBECTL get node "$NODE_NAME" >/dev/null 2>&1; then
              break
            fi
            sleep 2
          done

          $KUBECTL label node "$NODE_NAME" node.longhorn.io/create-default-disk=config --overwrite
          $KUBECTL annotate node "$NODE_NAME" node.longhorn.io/default-disks-config='${disksConfigJson}' --overwrite
        '';
      };
    })
  ];
}
