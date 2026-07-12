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

var k3sEnableFlannelCmd = &cobra.Command{
	Use:   "enable-flannel",
	Short: "Revert \"disable-flannel\" - restore K3s's default Flannel CNI",
	Long: `Removes the config.yaml override "disable-flannel" wrote, restoring K3s's
default Flannel CNI on next start, then restarts k3s on one node (--node)
or every node in infra.yaml (if --node is omitted).

Only meaningful if a replacement CNI (e.g. Cilium) was never actually
applied, or is being removed - if Cilium is already up and pods have been
recreated under it, reverting this without also uninstalling Cilium and
recreating pods again will leave the cluster in a mixed, broken state.

Example:
  homelab k3s enable-flannel
  homelab k3s enable-flannel --node pi-0`,
	RunE: runK3sEnableFlannel,
}

func init() {
	k3sEnableFlannelCmd.Flags().String("node", "", "Node name from infra.yaml (all nodes if omitted)")
	k3sCmd.AddCommand(k3sEnableFlannelCmd)
}

func runK3sEnableFlannel(cmd *cobra.Command, args []string) error {
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
		slog.Info("Re-enabling Flannel", "node", node.Name, "address", node.Address)

		client := ssh.NewClientWithPassword(node.Address, node.SSH.User, node.SSH.Password)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
		}

		err := k3s.EnableFlannel(client)
		client.Close()
		if err != nil {
			return fmt.Errorf("failed to re-enable Flannel on %s: %w", node.Name, err)
		}

		slog.Info("Flannel re-enabled, k3s restarted", "node", node.Name)
	}

	return nil
}

// selectNodes returns the nodes disable-flannel/enable-flannel should
// target: just the one named by --node, or every node in infra.yaml if
// --node is omitted.
func selectNodes(cmd *cobra.Command, infraCfg *config.InfraConfig) ([]config.NodeConfig, error) {
	nodeFlag, _ := cmd.Flags().GetString("node")
	if nodeFlag == "" {
		return infraCfg.Nodes, nil
	}

	for _, n := range infraCfg.Nodes {
		if n.Name == nodeFlag {
			return []config.NodeConfig{n}, nil
		}
	}
	return nil, fmt.Errorf("no node named %q in infra.yaml", nodeFlag)
}
