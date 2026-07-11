package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/liamawhite/homelab/pkg/config"
	"github.com/spf13/cobra"
)

var k3sCmd = &cobra.Command{
	Use:   "k3s",
	Short: "Install K3s on a node",
	Long: `Installs K3s on a provisioned node.

The node should be provisioned first using the 'bootstrap' command.

When joining (--server without --cluster-init), --token can be omitted: if
cluster.token isn't already set in infra.yaml, it's fetched automatically
by connecting to the node matching --server's host and saved back to
infra.yaml for future joins.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Examples:
  # First node (initialize cluster)
  homelab k3s --node pi-0 --cluster-init

  # Additional nodes (join cluster, token fetched automatically)
  homelab k3s --node pi-1 --server https://192.168.1.51:6443`,
	RunE: runK3s,
}

func init() {
	k3sCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
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

	// Joining a cluster requires a token; fetch and save it automatically
	// if it's not already set (via --token or infra.yaml's cluster.token).
	if !cfg.ClusterInit && cfg.Token == "" {
		token, err := fetchAndSaveClusterToken(ctx, cfg)
		if err != nil {
			return fmt.Errorf("no cluster token available and automatic fetch failed (use --token to provide one): %w", err)
		}
		cfg.Token = token
	}

	// Install K3s
	slog.Info("Installing K3s", "cluster_init", cfg.ClusterInit)
	installer := k3s.NewInstaller(client, cfg.K3SSANS)
	if err := installer.InstallK3s(ctx, cfg.ClusterInit, cfg.ServerURL, cfg.Token); err != nil {
		slog.Error("Failed to install K3s", "error", err)
		return err
	}

	// Extract kubeconfig if requested
	if cfg.OutputKubeconfig != "" {
		if cfg.InfraConfig == nil || cfg.InfraConfig.Cluster.VIP == "" {
			return fmt.Errorf("cluster.vip is not set in infra.yaml")
		}

		slog.Info("Extracting kubeconfig", "output", cfg.OutputKubeconfig)
		kubeconfig, err := k3s.ExtractKubeconfig(ctx, client, cfg.InfraConfig.Cluster.VIP)
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

// fetchAndSaveClusterToken connects to the node matching --server's host
// (looked up in infra.yaml by address) and extracts its cluster token,
// saving it to infra.yaml's cluster.token so future joins don't need to
// fetch it again.
func fetchAndSaveClusterToken(ctx context.Context, cfg *config.Config) (string, error) {
	if cfg.InfraConfig == nil {
		return "", fmt.Errorf("no infra.yaml loaded")
	}

	u, err := url.Parse(cfg.ServerURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("could not determine host from --server %q", cfg.ServerURL)
	}

	serverNode := config.FindNodeByAddress(cfg.InfraConfig, u.Hostname())
	if serverNode == nil {
		return "", fmt.Errorf("no node in infra.yaml matches --server host %s", u.Hostname())
	}

	slog.Info("Fetching cluster token from join server", "node", serverNode.Name, "address", serverNode.Address)

	client := ssh.NewClientWithPassword(serverNode.Address, serverNode.SSH.User, serverNode.SSH.Password)
	if err := client.Connect(ctx); err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", serverNode.Address, err)
	}
	defer client.Close()

	token, err := k3s.NewInstaller(client, nil).GetClusterToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to extract cluster token: %w", err)
	}

	if cfg.ConfigFile != "" {
		if err := config.SetClusterToken(cfg.ConfigFile, token); err != nil {
			return "", fmt.Errorf("failed to save token to %s: %w", cfg.ConfigFile, err)
		}
		slog.Info("Saved cluster token to infra.yaml", "path", cfg.ConfigFile)
	}

	return token, nil
}
