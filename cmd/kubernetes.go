package cmd

import (
	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s", "kube", "k"},
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(kubernetesCmd)
}
