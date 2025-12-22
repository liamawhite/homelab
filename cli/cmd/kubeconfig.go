package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/spf13/cobra"
)

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Extract kubeconfig from a K3s node",
	Long: `Extracts and displays kubeconfig from an existing K3s cluster node.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab kubeconfig --node pi-0`,
	RunE: runKubeconfig,
}

func init() {
	kubeconfigCmd.Flags().String("node", "", "Node name from infra.yaml (required)")
	kubeconfigCmd.Flags().String("ssh-user", "", "SSH username (optional if defined in infra.yaml)")
	kubeconfigCmd.Flags().String("output", "", "Path to write kubeconfig (default: stdout)")
}

func runKubeconfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration (skip K3s validation - we're just extracting kubeconfig)
	slog.Info("Loading configuration")
	cfg, err := config.LoadWithOptions(cmd, true)
	if err != nil {
		return err
	}

	output, _ := cmd.Flags().GetString("output")

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

	// Extract kubeconfig
	slog.Info("Extracting kubeconfig")
	kubeconfig, err := k3s.ExtractKubeconfig(ctx, client, cfg.Node)
	if err != nil {
		slog.Error("Failed to extract kubeconfig", "error", err)
		return err
	}

	if output != "" {
		if err := k3s.WriteKubeconfig(kubeconfig, output); err != nil {
			slog.Error("Failed to write kubeconfig", "error", err)
			return err
		}
	} else {
		fmt.Print(kubeconfig)
	}

	return nil
}
