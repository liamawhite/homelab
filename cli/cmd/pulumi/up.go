package pulumi

import (
	"context"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/spf13/cobra"
)

// UpCmd is the "up" command, deploying the Pulumi-managed infrastructure.
var UpCmd = &cobra.Command{
	Use:   "up",
	Short: "Deploy the Pulumi-managed infrastructure (kube-vip)",
	Long: `Resolves a reachable cluster endpoint - the VIP if kube-vip is already
running, otherwise trying each node directly, which is what makes the very
first "up" possible before kube-vip exists to serve the VIP - extracts a
kubeconfig from it, and runs the Pulumi program against that cluster.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab up`,
	RunE: runUp,
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx, timeout, stack, err := prepareStack(cmd)
	if err != nil {
		return err
	}

	upCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = stack.Up(upCtx, optup.ProgressStreams(os.Stdout))
	return err
}
