package raspberry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/liamawhite/homelab/cli/pkg/ssh"
)

type Provisioner struct {
	sshClient *ssh.Client
}

// NewProvisioner creates a new Raspberry Pi provisioner
func NewProvisioner(client *ssh.Client) *Provisioner {
	return &Provisioner{sshClient: client}
}

// Provision provisions a Raspberry Pi node
func (p *Provisioner) Provision(ctx context.Context) error {
	slog.Info("Starting Raspberry Pi provisioning")

	// 1. Copy config.txt
	slog.Info("Copying config.txt")
	if err := p.sshClient.WriteFile("/boot/firmware/config.txt", ConfigTxt, true); err != nil {
		return fmt.Errorf("failed to copy config.txt: %w", err)
	}

	// 2. Copy and apply eeprom.conf
	slog.Info("Copying and applying eeprom.conf")
	if err := p.sshClient.WriteFile("/boot/firmware/eeprom.conf", EepromConf, true); err != nil {
		return fmt.Errorf("failed to copy eeprom.conf: %w", err)
	}

	_, _, err := p.sshClient.ExecuteSudo("rpi-eeprom-config --apply /boot/firmware/eeprom.conf")
	if err != nil {
		return fmt.Errorf("failed to apply eeprom config: %w", err)
	}

	// 3. Update cmdline.txt
	slog.Info("Updating cmdline.txt with cgroup parameters")
	current, err := p.sshClient.ReadFile("/boot/firmware/cmdline.txt", true)
	if err != nil {
		return fmt.Errorf("failed to read cmdline.txt: %w", err)
	}

	updated := p.updateCmdline(current)
	if updated != current {
		if err := p.sshClient.WriteFile("/boot/firmware/cmdline.txt", updated, true); err != nil {
			return fmt.Errorf("failed to write cmdline.txt: %w", err)
		}
		slog.Info("Updated cmdline.txt with cgroup parameters")
	} else {
		slog.Info("cmdline.txt already has required cgroup parameters")
	}

	// 4. Install required packages
	slog.Info("Installing required packages", "packages", "open-iscsi, nfs-common")
	stdout, _, err := p.sshClient.ExecuteSudo("sh -c 'DEBIAN_FRONTEND=noninteractive apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y open-iscsi nfs-common'")
	if err != nil {
		slog.Error("Package installation failed", "output", stdout)
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// 5. Reboot and wait
	slog.Info("Rebooting node")
	if err := p.sshClient.Reboot(); err != nil {
		return fmt.Errorf("failed to reboot: %w", err)
	}

	slog.Info("Waiting for node to reboot", "timeout", "5m")
	if err := p.sshClient.WaitForReboot(5 * time.Minute); err != nil {
		return fmt.Errorf("failed to wait for reboot: %w", err)
	}

	slog.Info("Node successfully rebooted")
	return nil
}

// updateCmdline adds cgroup parameters to cmdline.txt if not present
func (p *Provisioner) updateCmdline(current string) string {
	current = strings.TrimSpace(current)

	if !strings.Contains(current, "cgroup_memory=1") {
		current += " cgroup_memory=1"
	}

	if !strings.Contains(current, "cgroup_enable=memory") {
		current += " cgroup_enable=memory"
	}

	return current
}
