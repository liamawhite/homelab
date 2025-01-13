package machine

import (
	"fmt"
	"os"

	"github.com/liamawhite/homelab/pkg/pi5"
	"github.com/liamawhite/homelab/pkg/remote"
	"github.com/spf13/cobra"
)

var pi5Cmd = &cobra.Command{
	Use:  "pi5 <username>@<address>",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := remote.NewClient(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer client.Close()
		if err := pi5.Bootstrap(client); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	bootstrapCmd.AddCommand(pi5Cmd)
}
