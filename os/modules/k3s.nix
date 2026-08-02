# Joins nix-gpu-3070-1 to the existing pi-0/pi-1/pi-2 cluster as an
# additional server (control-plane + etcd) node - every node in this
# cluster runs k3s in `server` role, there's no agent/worker split (see
# pkg/k3s/installer.go). Flags below mirror that installer's exactly:
#   - --disable-kube-proxy: Cilium's kubeProxyReplacement already handles
#     Service routing (pkg/components/cilium).
#   - --flannel-backend=none / --disable-network-policy: NOT in
#     pkg/k3s/installer.go's CLI flags, but confirmed live on pi-0's
#     /etc/rancher/k3s/config.yaml (set out-of-band, presumably alongside
#     the Cilium rollout). k3s treats these as "critical" values checked
#     against the cluster's bootstrap data on every join - omitting them
#     here made this node's k3s refuse to start with "critical
#     configuration value mismatch between servers". Re-verify against a
#     Pi's /etc/rancher/k3s/config.yaml if this node ever fails to join
#     again with the same error.
#
# Pinned to nixpkgs' `k3s_1_36` (currently 1.36.2+k3s1) rather than the
# unversioned `k3s` attribute (currently aliased to 1_35), to exactly match
# the Pi cluster's live version (installed via get.k3s.io, unpinned - re-check
# `kubectl get nodes -o wide` before bumping this). nixpkgs periodically
# retires old k3s_1_NN attrs as new minors land - if `k3s_1_36` ever
# disappears, re-check with `nix eval
# github:NixOS/nixpkgs/nixos-unstable#k3s_1_36.version` (or whatever the Pi
# cluster's version has become) and update the attribute name below.
{ pkgs, ... }:

{
  services.k3s = {
    enable = true;
    package = pkgs.k3s_1_36;
    role = "server";
    serverAddr = "https://192.168.1.50:6443";
    tokenFile = "/etc/k3s-token"; # staged via nixos-anywhere --extra-files, see os/README.md
    extraFlags = toString [
      "--disable=traefik"
      "--disable=servicelb"
      "--disable-kube-proxy"
      "--flannel-backend=none"
      "--disable-network-policy"
      "--tls-san=192.168.1.60"
      "--tls-san=192.168.1.50"
    ];
  };

  # NixOS's default host firewall filters INPUT on every interface,
  # including Cilium's virtual ones - kubelet-to-pod traffic (e.g. this
  # node's istio-cni readiness probe hitting the pod's own IP directly)
  # gets silently dropped rather than reaching the pod, since those
  # interfaces aren't trusted by default. Piecemeal-opening the k3s
  # control-plane ports (6443/2379/2380/10250) above isn't enough - Cilium
  # and every workload's own ports also need through. The Pi nodes (Debian)
  # don't run a host firewall at all; Cilium's own iptables/eBPF rules are
  # the only traffic control there - matching that instead of trying to
  # enumerate every port every component might need.
  networking.firewall.enable = false;
}
