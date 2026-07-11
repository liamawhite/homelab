package cmd

import (
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:     "node",
	Aliases: []string{"nodes", "n"},
	Short:   "Inspect nodes defined in infra.yaml",
}

func init() {
	nodeCmd.AddCommand(nodeStatusCmd)
}
