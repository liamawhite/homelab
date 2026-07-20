package lights

import (
	"github.com/spf13/cobra"
)

var hubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Discover and pair with Hue bridges",
}

func init() {
	hubCmd.AddCommand(hubLsCmd)
	hubCmd.AddCommand(hubPairCmd)
}
