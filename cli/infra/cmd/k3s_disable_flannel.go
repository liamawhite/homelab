package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/k3s"
	"github.com/liamawhite/homelab/pkg/ssh"
	"github.com/spf13/cobra"
)

var k3sDisableFlannelCmd = &cobra.Command{
	Use:   "disable-flannel",
	Short: "Disable K3s's built-in Flannel CNI, in preparation for installing a replacement",
	Long: `Disables K3s's built-in Flannel CNI and its network policy controller on
one node (--node) or every node in infra.yaml (if --node is omitted),
restarting k3s on each so the change takes effect.

This is step 1 of the Flannel-to-Cilium migration described in
pkg/components/cilium's package doc - it is only meaningful immediately
followed by "homelab up" (to install the replacement CNI) and a forced
recreation of every existing pod. Do not run this against a healthy
cluster with no replacement CNI ready to apply - pod networking breaks
the moment this completes, on every node it's run against.

Example:
  # Every node in infra.yaml, in sequence
  homelab k3s disable-flannel

  # A single node, e.g. to verify the command against one node first
  homelab k3s disable-flannel --node pi-0`,
	RunE: runK3sDisableFlannel,
}

func init() {
	k3sDisableFlannelCmd.Flags().String("node", "", "Node name from infra.yaml (all nodes if omitted)")
	k3sCmd.AddCommand(k3sDisableFlannelCmd)
}

func runK3sDisableFlannel(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	infraCfg, err := config.LoadInfra(cmd)
	if err != nil {
		return err
	}

	nodes, err := selectNodes(cmd, infraCfg)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		slog.Info("Disabling Flannel", "node", node.Name, "address", node.Address)

		client := ssh.NewClientWithPassword(node.Address, node.SSH.User, node.SSH.Password)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
		}

		err := k3s.DisableFlannel(client)
		client.Close()
		if err != nil {
			return fmt.Errorf("failed to disable Flannel on %s: %w", node.Name, err)
		}

		slog.Info("Flannel disabled, k3s restarted", "node", node.Name)
	}

	return nil
}
