package machine

import (
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use: "bootstrap",
}

func init() {
	MachineCmd.AddCommand(bootstrapCmd)
}
