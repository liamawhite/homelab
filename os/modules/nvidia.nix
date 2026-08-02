# NVIDIA RTX 3070 (Ampere) driver + container runtime wiring, so k3s's
# embedded containerd can hand GPUs to pods. This only makes the host
# capable - actually scheduling GPU pods still needs a cluster-side
# RuntimeClass + NVIDIA device-plugin DaemonSet (a pkg/components/ addition,
# not part of this OS config). See os/README.md's GPU section.
{ config, ... }:

{
  nixpkgs.config.allowUnfree = true; # NVIDIA driver package is unfree-redistributable

  hardware.graphics.enable = true;

  services.xserver.videoDrivers = [ "nvidia" ];

  hardware.nvidia = {
    open = true; # officially supported for Ampere/RTX 30-series
    modesetting.enable = true;
    package = config.boot.kernelPackages.nvidiaPackages.stable;
  };

  # Patches k3s's embedded containerd (via its config.toml template) to
  # register the nvidia runtime, so pods requesting a "nvidia" RuntimeClass
  # can actually reach the GPU. Verify this option still lives here in
  # nixpkgs-unstable at implementation/apply time - it has moved before.
  hardware.nvidia-container-toolkit.enable = true;
}
