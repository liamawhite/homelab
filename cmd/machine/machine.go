package machine

import (
	"github.com/spf13/cobra"
)

var MachineCmd = &cobra.Command{
	Use:   "machine",
	Short: "manage machines in my homelab",
}
