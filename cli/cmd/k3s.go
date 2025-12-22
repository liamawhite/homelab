package cmd

import (
	"context"
	"log/slog"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/spf13/cobra"
)

var k3sCmd = &cobra.Command{
	Use:   "k3s",
	Short: "Install K3s on a node",
	Long: `Installs K3s on a provisioned node.

The node should be provisioned first using the 'bootstrap' command.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Examples:
  # First node (initialize cluster)
  homelab k3s --node pi-0 --cluster-init

  # Additional nodes (join cluster)
  homelab k3s --node pi-1 --server https://192.168.1.51:6443 --token K10xxx...`,
	RunE: runK3s,
}

func init() {
	k3sCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
	k3sCmd.Flags().String("ssh-user", "", "SSH username (optional if defined in infra.yaml)")
	k3sCmd.Flags().StringSlice("sans", []string{}, "Additional TLS SANs for K3s API server (optional, K3s includes localhost/IPs by default)")
	k3sCmd.Flags().Bool("cluster-init", false, "Initialize new cluster")
	k3sCmd.Flags().String("server", "", "K3s server to join")
	k3sCmd.Flags().String("token", "", "K3s cluster token")
	k3sCmd.Flags().String("output-kubeconfig", "./kubeconfig", "Path to write kubeconfig")
}

func runK3s(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	slog.Info("Loading configuration")
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	slog.Info("Starting K3s installation", "node", cfg.Node)

	// Create SSH client
	var authMethod string
	if cfg.SSHPassword != "" {
		authMethod = "password"
	} else {
		authMethod = "key"
	}

	slog.Info("Creating SSH connection", "node", cfg.Node, "user", cfg.SSHUser, "auth_method", authMethod, "password_provided", cfg.SSHPassword != "")

	var client *ssh.Client
	client = ssh.NewClientWithPassword(cfg.Node, cfg.SSHUser, cfg.SSHPassword)

	if err := client.Connect(ctx); err != nil {
		slog.Error("SSH connection failed", "node", cfg.Node, "user", cfg.SSHUser, "auth_method", authMethod, "error", err)
		return err
	}
	defer client.Close()

	slog.Info("Successfully connected to node")

	// Install K3s
	slog.Info("Installing K3s", "cluster_init", cfg.ClusterInit)
	installer := k3s.NewInstaller(client, cfg.K3SSANS)
	if err := installer.InstallK3s(ctx, cfg.ClusterInit, cfg.ServerURL, cfg.Token); err != nil {
		slog.Error("Failed to install K3s", "error", err)
		return err
	}

	// Extract kubeconfig if requested
	if cfg.OutputKubeconfig != "" {
		slog.Info("Extracting kubeconfig", "output", cfg.OutputKubeconfig)
		kubeconfig, err := k3s.ExtractKubeconfig(ctx, client, cfg.Node)
		if err != nil {
			slog.Error("Failed to extract kubeconfig", "error", err)
			return err
		}

		if err := k3s.WriteKubeconfig(kubeconfig, cfg.OutputKubeconfig); err != nil {
			slog.Error("Failed to write kubeconfig", "error", err)
			return err
		}
	}

	slog.Info("K3s installation complete", "node", cfg.Node)
	return nil
}
