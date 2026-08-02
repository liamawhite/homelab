# Disk layout for nix-gpu-3070-1. Device paths confirmed 2026-08-01 via
# `ls -la /dev/disk/by-id/` on the live NixOS USB installer: nvme is the
# Samsung 980 PRO 1TB, hdd is the Seagate ST2000DM008 2TB. by-id paths are
# used (not /dev/sda/nvme0n1 etc.) because /dev/sdX-style names aren't
# guaranteed stable across boots/disk enumeration order. (sdb on the live
# installer is the USB stick itself - not one of these two.)
{
  disko.devices = {
    disk = {
      nvme = {
        device = "/dev/disk/by-id/nvme-eui.002538b121411a78";
        type = "disk";
        content = {
          type = "gpt";
          partitions = {
            ESP = {
              size = "512M";
              type = "EF00";
              content = {
                type = "filesystem";
                format = "vfat";
                mountpoint = "/boot";
                mountOptions = [ "umask=0077" ];
              };
            };
            root = {
              size = "100%";
              content = {
                type = "filesystem";
                format = "ext4";
                mountpoint = "/";
              };
            };
          };
        };
      };

      hdd = {
        device = "/dev/disk/by-id/ata-ST2000DM008-2FR102_ZFL5XFJC";
        type = "disk";
        content = {
          type = "gpt";
          partitions.storage = {
            size = "100%";
            content = {
              type = "filesystem";
              format = "ext4";
              mountpoint = "/mnt/storage";
            };
          };
        };
      };
    };
  };
}
