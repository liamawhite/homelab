package pulumi

import (
	"context"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/spf13/cobra"
)

// RefreshCmd is a TEMPORARY one-off command to reconcile Pulumi state with
// the live cluster after a resource rename caused two logical resources to
// collide on one physical CiliumClusterwideNetworkPolicy object name, and
// the delete of the old one silently removed what the new one had just
// created - not meant to be a permanent CLI command.
var RefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "TEMPORARY: reconcile Pulumi state with the live cluster",
	RunE:  runRefresh,
}

func runRefresh(cmd *cobra.Command, args []string) error {
	ctx, timeout, stack, err := prepareStack(cmd)
	if err != nil {
		return err
	}

	refreshCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = stack.Refresh(refreshCtx, optrefresh.ProgressStreams(os.Stdout))
	return err
}
