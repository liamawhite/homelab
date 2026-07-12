package cmd

import (
	"context"
	"log/slog"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/raspberry"
	"github.com/liamawhite/homelab/pkg/ssh"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Provision a Raspberry Pi node",
	Long: `Provisions a Raspberry Pi node with required boot configuration and packages.

This prepares the Pi for K3s installation but does not install K3s.
After bootstrapping, use the 'k3s' command to install K3s.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab bootstrap --node pi-0`,
	RunE: runBootstrap,
}

func init() {
	bootstrapCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration (skip K3s validation - bootstrap doesn't touch K3s)
	slog.Info("Loading configuration")
	cfg, err := config.LoadWithOptions(cmd, true)
	if err != nil {
		return err
	}

	slog.Info("Starting Raspberry Pi provisioning", "node", cfg.Node)

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

	// Provision Raspberry Pi
	slog.Info("Provisioning Raspberry Pi")
	provisioner := raspberry.NewProvisioner(client)
	if err := provisioner.Provision(ctx); err != nil {
		slog.Error("Failed to provision Raspberry Pi", "error", err)
		return err
	}

	slog.Info("Raspberry Pi provisioning complete", "node", cfg.Node)
	return nil
}
