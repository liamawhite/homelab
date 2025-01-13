package kubernetes

import (
	"fmt"
	"os"

	"github.com/liamawhite/homelab/pkg/kubernetes"
	"github.com/liamawhite/homelab/pkg/remote"
	"github.com/spf13/cobra"
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:  "kubeconfig <username>@<address>",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := remote.NewClient(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer client.Close()
		if err := kubernetes.RetrieveKubeConfig(client); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	KubernetesCmd.AddCommand(kubeconfigCmd)
}
