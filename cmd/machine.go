package cmd

import (
	"github.com/spf13/cobra"
)

var machineCmd = &cobra.Command{
	Use:   "machine",
	Short: "manage machines in my homelab",
}

func init() {
	rootCmd.AddCommand(machineCmd)
}
