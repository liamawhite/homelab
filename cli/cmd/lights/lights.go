// Package lights implements the "lights" command group: discovering and
// pairing with Philips Hue bridges, and listing the actual lights they
// control.
package lights

import (
	"github.com/spf13/cobra"
)

// Cmd is the "lights" command, added directly to the root command.
var Cmd = &cobra.Command{
	Use:   "lights",
	Short: "Discover and manage smart lights",
}

func init() {
	Cmd.AddCommand(hubCmd)
	Cmd.AddCommand(lsCmd)
	Cmd.AddCommand(switchesCmd)
}
