package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/spf13/cobra"
)

var clustertokenCmd = &cobra.Command{
	Use:   "clustertoken",
	Short: "Extract cluster token from a K3s node",
	Long: `Extracts and saves the cluster token from an existing K3s cluster node.

This token is used by other nodes to join the cluster.
The token will be saved to a file (default: ./cluster-token).

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab clustertoken --node pi-0
  homelab clustertoken --node pi-0 --output /path/to/token`,
	RunE: runClusterToken,
}

func init() {
	clustertokenCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
	clustertokenCmd.Flags().String("ssh-user", "", "SSH username (optional if defined in infra.yaml)")
	clustertokenCmd.Flags().String("output", "./cluster-token", "Path to write cluster token (default: ./cluster-token)")
}

func runClusterToken(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration (skip K3s validation - we're just extracting a token)
	slog.Info("Loading configuration")
	cfg, err := config.LoadWithOptions(cmd, true)
	if err != nil {
		return err
	}

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

	// Extract token
	slog.Info("Extracting cluster token")
	installer := k3s.NewInstaller(client, nil)
	token, err := installer.GetClusterToken(ctx)
	if err != nil {
		slog.Error("Failed to get cluster token", "error", err)
		return err
	}

	// Get output path
	output, _ := cmd.Flags().GetString("output")

	// Write token to file
	slog.Info("Saving cluster token", "output", output)
	if err := os.WriteFile(output, []byte(token), 0600); err != nil {
		slog.Error("Failed to write token file", "output", output, "error", err)
		return fmt.Errorf("failed to write token to %s: %w", output, err)
	}

	fmt.Printf("Cluster token saved to: %s\n", output)

	return nil
}
