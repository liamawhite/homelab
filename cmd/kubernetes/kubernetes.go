package kubernetes

import (
	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var KubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s", "kube", "k"},
	Run: func(cmd *cobra.Command, args []string) {
	},
}
