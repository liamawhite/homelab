package lights

import (
	"github.com/spf13/cobra"
)

var switchesCmd = &cobra.Command{
	Use:     "switches",
	Aliases: []string{"switch", "sw"},
	Short:   "List physical Hue switches (dimmer, tap, wall switches)",
}

func init() {
	switchesCmd.AddCommand(switchesLsCmd)
}
