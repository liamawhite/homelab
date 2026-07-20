package lights

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/liamawhite/homelab/pkg/lights/hue"
	"github.com/spf13/cobra"
)

var hubLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List Philips Hue bridges on the local network",
	Long: `Finds Hue bridges using cloud (N-UPnP) and/or local (SSDP) discovery
(see --bridge-discovery-method), then queries each candidate's
unauthenticated /api/config endpoint to verify it is a genuine bridge and
enrich the listing with its name, model, and firmware/API versions.

Example:
  homelab lights hub ls`,
	RunE: runHubLs,
}

func init() {
	hubLsCmd.Flags().Duration("timeout", 5*time.Second, "How long to wait for discovery responses")
	addMethodFlag(hubLsCmd)
}

func runHubLs(cmd *cobra.Command, args []string) error {
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return fmt.Errorf("failed to read --timeout flag: %w", err)
	}
	method, err := methodFlag(cmd)
	if err != nil {
		return err
	}

	bridges, err := hue.Discover(cmd.Context(), timeout, method)
	if err != nil {
		return err
	}

	if len(bridges) == 0 {
		fmt.Println("No Hue bridges found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tID\tIP\tMODEL\tAPI VERSION\tSW VERSION\tMAC")
	for _, b := range bridges {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", b.Name, b.ID, b.IP, b.ModelID, b.APIVersion, b.SWVersion, b.MAC)
	}
	return w.Flush()
}
