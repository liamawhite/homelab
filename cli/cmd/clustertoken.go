package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/liamawhite/homelab/cli/pkg/config"
	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/spf13/cobra"
)

var clustertokenCmd = &cobra.Command{
	Use:   "clustertoken",
	Short: "Extract cluster token from a K3s node",
	Long: `Extracts and displays the cluster token from an existing K3s cluster node.

This token is used by other nodes to join the cluster.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab clustertoken --node pi-0`,
	RunE: runClusterToken,
}

func init() {
	clustertokenCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
	clustertokenCmd.Flags().String("ssh-user", "", "SSH username (optional if defined in infra.yaml)")
}

func runClusterToken(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration (handles node-name, infra.yaml, env vars, etc.)
	slog.Info("Loading configuration")
	cfg, err := config.Load(cmd)
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
	fmt.Println(token)

	return nil
}
