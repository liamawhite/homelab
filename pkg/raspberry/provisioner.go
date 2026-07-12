package raspberry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/liamawhite/homelab/pkg/ssh"
)

type Provisioner struct {
	sshClient *ssh.Client
}

// NewProvisioner creates a new Raspberry Pi provisioner
func NewProvisioner(client *ssh.Client) *Provisioner {
	return &Provisioner{sshClient: client}
}

// Provision provisions a Raspberry Pi node. It is safe to run repeatedly:
// files are only rewritten and the node only rebooted if something actually
// changed.
func (p *Provisioner) Provision(ctx context.Context) error {
	slog.Info("Starting Raspberry Pi provisioning")

	needsReboot := false

	// 1. Copy config.txt (only if it differs from what's already there)
	changed, err := p.writeIfChanged("/boot/firmware/config.txt", ConfigTxt)
	if err != nil {
		return fmt.Errorf("failed to copy config.txt: %w", err)
	}
	needsReboot = needsReboot || changed

	// 2. Copy and apply eeprom.conf (only if it differs)
	changed, err = p.writeIfChanged("/boot/firmware/eeprom.conf", EepromConf)
	if err != nil {
		return fmt.Errorf("failed to copy eeprom.conf: %w", err)
	}
	if changed {
		slog.Info("Applying eeprom.conf")
		if _, _, err := p.sshClient.ExecuteSudo("rpi-eeprom-config --apply /boot/firmware/eeprom.conf"); err != nil {
			return fmt.Errorf("failed to apply eeprom config: %w", err)
		}
		needsReboot = true
	} else {
		slog.Info("eeprom.conf already up to date")
	}

	// 3. Update cmdline.txt
	slog.Info("Updating cmdline.txt with cgroup parameters")
	current, err := p.sshClient.ReadFile("/boot/firmware/cmdline.txt", true)
	if err != nil {
		return fmt.Errorf("failed to read cmdline.txt: %w", err)
	}

	updated := p.updateCmdline(current)
	if updated != strings.TrimSpace(current) {
		if err := p.sshClient.WriteFile("/boot/firmware/cmdline.txt", updated, true); err != nil {
			return fmt.Errorf("failed to write cmdline.txt: %w", err)
		}
		slog.Info("Updated cmdline.txt with cgroup parameters")
		needsReboot = true
	} else {
		slog.Info("cmdline.txt already has required cgroup parameters")
	}

	// 4. Install required packages (apt-get is already idempotent)
	slog.Info("Installing required packages", "packages", "open-iscsi, nfs-common")
	stdout, _, err := p.sshClient.ExecuteSudo("sh -c 'DEBIAN_FRONTEND=noninteractive apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y open-iscsi nfs-common'")
	if err != nil {
		slog.Error("Package installation failed", "output", stdout)
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// 5. Reboot and wait, but only if something that requires it changed
	if !needsReboot {
		slog.Info("No changes requiring a reboot; provisioning already up to date")
		return nil
	}

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

// writeIfChanged writes content to path only if it differs from what's
// already there, reporting whether a write happened.
func (p *Provisioner) writeIfChanged(path, content string) (bool, error) {
	if matches, err := p.fileMatches(path, content); err != nil {
		slog.Debug("Could not read existing file, will write it", "path", path, "error", err)
	} else if matches {
		return false, nil
	}

	if err := p.sshClient.WriteFile(path, content, true); err != nil {
		return false, err
	}
	return true, nil
}

// fileMatches reports whether the file at path already has the given
// content. Shared by writeIfChanged (to decide what Provision needs to
// change) and CheckBootstrapped (to report status without changing anything).
func (p *Provisioner) fileMatches(path, content string) (bool, error) {
	current, err := p.sshClient.ReadFile(path, true)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(current) == strings.TrimSpace(content), nil
}

// BootstrapStatus reports whether each part of Raspberry Pi provisioning is
// already in place.
type BootstrapStatus struct {
	ConfigTxt  bool
	EepromConf bool
	Cmdline    bool
	Packages   bool
}

// Bootstrapped reports whether every part of provisioning is already done.
func (s BootstrapStatus) Bootstrapped() bool {
	return s.ConfigTxt && s.EepromConf && s.Cmdline && s.Packages
}

// CheckBootstrapped inspects the node's current state without changing
// anything, reusing the same comparisons Provision uses to decide what
// needs to change.
func (p *Provisioner) CheckBootstrapped() BootstrapStatus {
	var status BootstrapStatus

	status.ConfigTxt, _ = p.fileMatches("/boot/firmware/config.txt", ConfigTxt)
	status.EepromConf, _ = p.fileMatches("/boot/firmware/eeprom.conf", EepromConf)

	if current, err := p.sshClient.ReadFile("/boot/firmware/cmdline.txt", true); err == nil {
		status.Cmdline = p.updateCmdline(current) == strings.TrimSpace(current)
	}

	if _, _, err := p.sshClient.Execute("dpkg -s open-iscsi nfs-common"); err == nil {
		status.Packages = true
	}

	return status
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
