package cmd

import (
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use: "bootstrap",
}

func init() {
	machineCmd.AddCommand(bootstrapCmd)
}
