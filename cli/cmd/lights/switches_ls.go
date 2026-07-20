package lights

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/lights/hue"
	"github.com/spf13/cobra"
)

var switchesLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List buttons on paired Hue switches",
	Long: `Lists every button on every physical switch (dimmer switch, tap
switch, wall switch) on each bridge saved in infra.yaml's
lights.hue.bridges (see 'homelab lights hub pair'). Unlike a light, a
switch has no ongoing state - only the most recent event it reported and
its battery level.

By default, the bridge ID and button ID columns are hidden - pass
'-o wide' to include them.

Example:
  homelab lights switches ls`,
	RunE: runSwitchesLs,
}

func init() {
	switchesLsCmd.Flags().StringP("output", "o", "", "Output format. One of: wide")
	addMethodFlag(switchesLsCmd)
}

func runSwitchesLs(cmd *cobra.Command, args []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("failed to read --output flag: %w", err)
	}
	wide := output == "wide"
	method, err := methodFlag(cmd)
	if err != nil {
		return err
	}

	infraCfg, err := config.LoadInfra(cmd)
	if err != nil {
		return err
	}

	if len(infraCfg.Lights.Hue.Bridges) == 0 {
		fmt.Println("No paired bridges found. Run 'homelab lights hub pair' first.")
		return nil
	}

	bridges, err := hue.Discover(cmd.Context(), 5*time.Second, method)
	if err != nil {
		return err
	}
	ipByID := make(map[string]string, len(bridges))
	for _, b := range bridges {
		ipByID[b.ID] = b.IP
	}

	var allSwitches []hue.Switch
	for _, paired := range infraCfg.Lights.Hue.Bridges {
		ip, ok := ipByID[paired.ID]
		if !ok {
			slog.Warn("Paired bridge not found on the network", "id", paired.ID)
			continue
		}

		switches, err := hue.FetchSwitches(cmd.Context(), ip, paired.ID, paired.AppKey)
		if err != nil {
			slog.Warn("Failed to fetch switches from bridge", "id", paired.ID, "error", err)
			continue
		}
		allSwitches = append(allSwitches, switches...)
	}
	sort.Slice(allSwitches, func(i, j int) bool {
		if allSwitches[i].Name != allSwitches[j].Name {
			return allSwitches[i].Name < allSwitches[j].Name
		}
		return allSwitches[i].ControlID < allSwitches[j].ControlID
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if wide {
		fmt.Fprintln(w, "NAME\tBUTTON\tLAST EVENT\tLAST EVENT TIME\tBATTERY\tPRODUCT\tMODEL\tBRIDGE\tID")
	} else {
		fmt.Fprintln(w, "NAME\tBUTTON\tLAST EVENT\tLAST EVENT TIME\tBATTERY\tPRODUCT\tMODEL")
	}

	for _, s := range allSwitches {
		lastEvent := dashIfEmpty(s.LastEvent, s.LastEvent == "")
		lastEventTime := dashIfEmpty(relativeTime(s.LastEventTime), s.LastEventTime.IsZero())
		battery := dashIfEmpty(fmt.Sprintf("%d%%", s.Battery), s.Battery < 0)
		product := dashIfEmpty(s.Product, s.Product == "")
		model := dashIfEmpty(s.Model, s.Model == "")

		if wide {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Name, s.ControlID, lastEvent, lastEventTime, battery, product, model, s.BridgeID, s.ID)
		} else {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
				s.Name, s.ControlID, lastEvent, lastEventTime, battery, product, model)
		}
	}
	return w.Flush()
}

// relativeTime renders t as a short human-readable relative duration, e.g.
// "5m ago", "3h ago", "2d ago".
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
