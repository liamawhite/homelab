package pulumi

import (
	"context"

	"github.com/spf13/cobra"
)

// CancelCmd clears a stuck update lock left behind by an interrupted "up"
// (e.g. after a Ctrl-C or killed process), so the next "up"/"preview" isn't
// blocked with a "stack is currently locked" error.
var CancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Clear a stuck Pulumi update lock left by an interrupted up/preview",
	RunE:  runCancel,
}

func runCancel(cmd *cobra.Command, args []string) error {
	ctx, timeout, stack, err := prepareStack(cmd)
	if err != nil {
		return err
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return stack.Cancel(cancelCtx)
}
