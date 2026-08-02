# nix-gpu-3070-1: ASUS ROG Strix desktop (RTX 3070), k3s control-plane node
# joining pi-0/pi-1/pi-2's cluster. See os/README.md for the install flow.
{ ... }:

{
  imports = [
    ./disko.nix
    ./hardware-configuration.nix
    # nvidia.nix deliberately NOT imported for the initial install: building
    # it (NVIDIA kernel module + full graphics stack) crashed the RAM-backed
    # live installer twice in a row at the same step (nvidia-x11 + initrd
    # build), with zero swap to cushion it. Get the base system installed
    # and booted first (real zram swap, no tmpfs constraints), then add
    # this back via `nixos-rebuild switch` - see os/README.md.
    ../../modules/k3s.nix
    ../../modules/longhorn.nix
  ];

  networking.hostName = "nix-gpu-3070-1";

  # eno1 confirmed 2026-08-01 via `ip -brief link` on the live NixOS USB
  # installer - it's the wired NIC (UP/LOWER_UP); wlp5s0 is wifi, unused.
  #
  # DHCP (not a hardcoded static address) deliberately, matching the Pi
  # nodes' convention - they get 192.168.1.51/.52/.53 from UniFi's own DHCP
  # server via a reservation, not an OS-level static config. This node
  # originally hardcoded 192.168.1.60 directly instead, which meant UniFi's
  # DHCP server never issued that lease and had no DHCP-snooped binding for
  # it - a likely cause of one LAN client (a Mac on the same switch/subnet)
  # being unable to complete TCP handshakes to this node (SYN arrived, but
  # the SYN-ACK reply back never did) while everything else worked fine,
  # consistent with DHCP-snooping/IP-Source-Guard-style validation on the
  # switch distrusting an IP with no known DHCP binding for this MAC. A
  # UniFi DHCP reservation for this MAC -> 192.168.1.60 already exists
  # (set up alongside the initial install), so this should still land on
  # the same address.
  networking.useDHCP = false;
  networking.interfaces.eno1.useDHCP = true;

  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;

  # No swap partition (see disko.nix) - zram is enough for a "basic OS".
  zramSwap.enable = true;

  # Registers the second physical disk (/mnt/storage, see disko.nix) with
  # Longhorn via the proper first-registration mechanism (label + JSON
  # annotation on this Node object, set by modules/longhorn.nix's systemd
  # oneshot) instead of the Pulumi Patch this node used until now - see
  # os/README.md's "Multi-disk nodes and Longhorn" section.
  homelab.longhorn.extraDisks = [ "/mnt/storage" ];

  time.timeZone = "America/Los_Angeles";

  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  users.users.liam = {
    isNormalUser = true;
    extraGroups = [ "wheel" ];
    openssh.authorizedKeys.keyFiles = [ ../../ssh/nix-gpu-3070-1.pub ];
  };
  security.sudo.wheelNeedsPassword = false;

  services.openssh = {
    enable = true;
    settings = {
      PasswordAuthentication = false;
      PermitRootLogin = "prohibit-password";
    };
  };

  system.stateVersion = "25.05";
}
