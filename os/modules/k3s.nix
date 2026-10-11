# Host prerequisites only. Ansible owns the K3s package root, service, and
# configuration; NixOS must not declare services.k3s or overwrite that unit.
{ pkgs, ... }:

{
  environment.systemPackages = with pkgs; [
    python3
    util-linux
    iproute2
    iptables
    conntrack-tools
    socat
  ];

  boot.kernelModules = [ "overlay" "br_netfilter" ];
  boot.kernel.sysctl = {
    "net.ipv4.ip_forward" = 1;
    "net.bridge.bridge-nf-call-iptables" = 1;
  };

  # /etc/systemd/system points into the immutable Nix store. A generator
  # exposes Ansible's persistent unit on boot and daemon-reload, including
  # after a NixOS switch. The package is separately GC-rooted by Ansible.
  systemd.generators.homelab-k3s = pkgs.writeShellScript "homelab-k3s" ''
    unit=/var/lib/homelab/systemd/k3s.service
    if [ -f "$unit" ]; then
      ${pkgs.coreutils}/bin/mkdir -p "$1/multi-user.target.wants"
      ${pkgs.coreutils}/bin/ln -sf "$unit" "$1/k3s.service"
      ${pkgs.coreutils}/bin/ln -sf ../k3s.service "$1/multi-user.target.wants/k3s.service"
    fi
  '';

  # Preserve the current host networking behavior. Cilium policies belong
  # to the cluster layer; a later firewall redesign is a separate change.
  networking.firewall.enable = false;
}
