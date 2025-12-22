package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/liamawhite/homelab/cli/pkg/ssh"
)

type Installer struct {
	sshClient *ssh.Client
	sans      []string
}

// NewInstaller creates a new K3s installer
func NewInstaller(client *ssh.Client, sans []string) *Installer {
	return &Installer{
		sshClient: client,
		sans:      sans,
	}
}

// InstallK3s installs K3s on the node
func (i *Installer) InstallK3s(ctx context.Context, clusterInit bool, serverURL, token string) error {
	slog.Info("Starting K3s installation")

	// Build and execute install command
	installCmd := i.buildInstallCommand(clusterInit, serverURL, token)
	slog.Info("Running K3s install script", "cluster_init", clusterInit)

	stdout, _, err := i.sshClient.Execute(installCmd)
	if err != nil {
		slog.Error("K3s installation failed", "error", err, "output", stdout)
		return fmt.Errorf("failed to install K3s: %w", err)
	}

	slog.Info("K3s installation completed successfully")
	return nil
}

// GetClusterToken retrieves the K3s cluster token
func (i *Installer) GetClusterToken(ctx context.Context) (string, error) {
	token, err := i.sshClient.ReadFile("/var/lib/rancher/k3s/server/token", true)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster token: %w", err)
	}
	return strings.TrimSpace(token), nil
}

// buildInstallCommand builds the K3s install command based on configuration
func (i *Installer) buildInstallCommand(clusterInit bool, serverURL, token string) string {
	// Base command
	cmd := "curl -sfL https://get.k3s.io | sh -s - server --disable=traefik --disable=servicelb"

	// Add TLS SANs
	for _, san := range i.sans {
		cmd += fmt.Sprintf(" --tls-san %s", san)
	}

	// Add cluster-specific flags
	if clusterInit {
		cmd += " --cluster-init"
	} else if serverURL != "" && token != "" {
		cmd += fmt.Sprintf(" --server %s --token %s", serverURL, token)
	}

	return cmd
}
