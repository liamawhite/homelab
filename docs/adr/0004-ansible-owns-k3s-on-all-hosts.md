# Give Ansible ownership of K3s on all hosts

Ansible will own K3s installation, configuration, service management, and upgrades on both Raspberry Pi OS and NixOS. Remove the existing NixOS K3s service declaration to provide one lifecycle owner and version source; the user explicitly permits this narrow change to the original OS boundary. NixOS continues to supply host prerequisites, and implementation must verify runtime compatibility and survival across NixOS rebuilds, including existing Longhorn integration.
