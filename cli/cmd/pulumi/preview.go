package pulumi

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/spf13/cobra"
)

// PreviewCmd is the "preview" command, previewing changes to the
// Pulumi-managed infrastructure.
var PreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview changes to the Pulumi-managed infrastructure (kube-vip)",
	Long: `Resolves a reachable cluster endpoint - the VIP if kube-vip is already
running, otherwise trying each node directly, which is what makes previewing
possible before kube-vip exists to serve the VIP - extracts a kubeconfig
from it, and previews the Pulumi program's changes against that cluster.

Configuration can be provided via:
1. infra.yaml file (see cli/infra.yaml for example)
2. Command-line flags (see below)

Example:
  homelab preview`,
	RunE: runPreview,
}

func runPreview(cmd *cobra.Command, args []string) error {
	ctx, stack, err := prepareStack(cmd)
	if err != nil {
		return err
	}

	_, err = stack.Preview(ctx, optpreview.ProgressStreams(os.Stdout))
	return err
}
