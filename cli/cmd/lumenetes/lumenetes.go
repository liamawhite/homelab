// Package lumenetes implements the "lumenetes" command group: discovering
// and pairing with Philips Hue bridges, and listing the actual lights they
// control.
package lumenetes

import (
	"github.com/spf13/cobra"
)

// Cmd is the "lumenetes" command, added directly to the root command.
var Cmd = &cobra.Command{
	Use:   "lumenetes",
	Short: "Discover and manage smart lights",
}

func init() {
	Cmd.AddCommand(hubCmd)
	Cmd.AddCommand(lsCmd)
	Cmd.AddCommand(switchesCmd)
}
