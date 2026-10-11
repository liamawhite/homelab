# Host prerequisites only. Ansible owns the K3s package root, service, and
# configuration; NixOS must not declare services.k3s or overwrite that unit.
{ pkgs, ... }:

{
  environment.systemPackages = with pkgs; [
    python3
    util-linux
    iproute2
    iptables
    nftables
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
    for entry in "k3s.service:multi-user.target" "homelab-host-firewall.service:network-pre.target" "homelab-firewall-dns.timer:timers.target"; do
      unit="''${entry%:*}"
      target="''${entry#*:}"
      if [ -f "/var/lib/homelab/systemd/$unit" ]; then
        ${pkgs.coreutils}/bin/mkdir -p "$1/$target.wants"
        ${pkgs.coreutils}/bin/ln -sf "/var/lib/homelab/systemd/$unit" "$1/$unit"
        ${pkgs.coreutils}/bin/ln -sf "../$unit" "$1/$target.wants/$unit"
      fi
    done
    if [ -f /var/lib/homelab/systemd/homelab-firewall-dns.service ]; then
      ${pkgs.coreutils}/bin/ln -sf /var/lib/homelab/systemd/homelab-firewall-dns.service "$1/homelab-firewall-dns.service"
    fi
  '';

  # Ansible owns the persistent homelab_host nftables table and its boot unit.
  # Avoid a second OS firewall manager replacing K3s/Cilium rules.
  networking.firewall.enable = false;
}
