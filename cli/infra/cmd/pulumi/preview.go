package pulumi

import (
	"context"
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
	ctx, timeout, stack, err := prepareStack(cmd)
	if err != nil {
		return err
	}

	previewCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = stack.Preview(previewCtx, optpreview.ProgressStreams(os.Stdout))
	return err
}
