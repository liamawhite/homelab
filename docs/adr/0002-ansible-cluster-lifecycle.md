# Use Ansible for cluster initialization and upgrades

Use Ansible playbooks to coordinate K3s initialization and upgrades across the Raspberry Pi and NixOS hosts, retaining their existing operating systems. This avoids maintaining custom SSH orchestration or introducing a management cluster and host-agent control system. Playbooks must accommodate the Nix server's existing declarative service ownership; that integration remains to be designed.
