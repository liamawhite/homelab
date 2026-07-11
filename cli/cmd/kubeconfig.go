package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/liamawhite/homelab/cli/pkg/k3s"
	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"github.com/liamawhite/homelab/pkg/config"
	"github.com/spf13/cobra"
)

const homelabContextName = "homelab"

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Extract kubeconfig from a K3s node",
	Long: `Extracts kubeconfig from an existing K3s cluster node and, by default,
merges it into your default kubeconfig (respecting $KUBECONFIG, else
~/.kube/config) under the "homelab" context, replacing any existing entry
of that name, and switches to it as the active context.

If --node is omitted, connects via the cluster VIP instead of a specific
node (using the first node's infra.yaml SSH credentials) - kube-vip routes
this to whichever node currently holds the VIP.

Use --output to instead write a standalone kubeconfig file (use "-" for
stdout).

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Examples:
  homelab kubeconfig --node pi-0
  homelab kubeconfig`,
	RunE: runKubeconfig,
}

func init() {
	kubeconfigCmd.Flags().String("node", "", "Node name from infra.yaml (default: connect via the cluster VIP)")
	kubeconfigCmd.Flags().String("output", "", "Path to write a standalone kubeconfig file, or \"-\" for stdout (default: merge into your default kubeconfig)")
}

func runKubeconfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	slog.Info("Loading configuration")
	infraCfg, err := config.LoadInfra(cmd)
	if err != nil {
		return err
	}
	if infraCfg.Cluster.VIP == "" {
		return fmt.Errorf("cluster.vip is not set in infra.yaml")
	}

	nodeName, _ := cmd.Flags().GetString("node")

	var targetAddr, sshUser, sshPassword string
	if nodeName == "" {
		if len(infraCfg.Nodes) == 0 {
			return fmt.Errorf("no nodes defined in infra.yaml")
		}
		slog.Info("No --node given, connecting via the cluster VIP", "vip", infraCfg.Cluster.VIP)
		targetAddr = infraCfg.Cluster.VIP
		sshUser = infraCfg.Nodes[0].SSH.User
		sshPassword = infraCfg.Nodes[0].SSH.Password
	} else {
		node := config.FindNodeByName(infraCfg, nodeName)
		if node == nil {
			return fmt.Errorf("node '%s' not found in config file", nodeName)
		}
		targetAddr = node.Address
		sshUser = node.SSH.User
		sshPassword = node.SSH.Password
	}

	output, _ := cmd.Flags().GetString("output")

	slog.Info("Creating SSH connection", "target", targetAddr, "user", sshUser)

	client := ssh.NewClientWithPassword(targetAddr, sshUser, sshPassword)
	if err := client.Connect(ctx); err != nil {
		slog.Error("SSH connection failed", "target", targetAddr, "error", err)
		return err
	}
	defer client.Close()

	// Extract kubeconfig
	slog.Info("Extracting kubeconfig")
	kubeconfig, err := k3s.ExtractKubeconfig(ctx, client, infraCfg.Cluster.VIP)
	if err != nil {
		slog.Error("Failed to extract kubeconfig", "error", err)
		return err
	}

	switch output {
	case "-":
		fmt.Print(kubeconfig)
	case "":
		target, err := defaultKubeconfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine default kubeconfig path: %w", err)
		}

		slog.Info("Merging kubeconfig", "path", target, "context", homelabContextName)
		if err := k3s.MergeKubeconfig(kubeconfig, target, homelabContextName); err != nil {
			slog.Error("Failed to merge kubeconfig", "error", err)
			return err
		}

		fmt.Printf("Merged into %s and switched to context %q\n", target, homelabContextName)
	default:
		if err := k3s.WriteKubeconfig(kubeconfig, output); err != nil {
			slog.Error("Failed to write kubeconfig", "error", err)
			return err
		}
	}

	return nil
}

// defaultKubeconfigPath resolves where kubectl would look for its
// kubeconfig: $KUBECONFIG (its first entry, if it lists several), else
// ~/.kube/config.
func defaultKubeconfigPath() (string, error) {
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		return strings.Split(kc, string(os.PathListSeparator))[0], nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "config"), nil
}
