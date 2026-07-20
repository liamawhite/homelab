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

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List lights known to paired Hue bridges",
	Long: `Lists every light on each bridge saved in infra.yaml's
lights.hue.bridges (see 'homelab lights hub pair'). Bridges are
re-discovered on the network each run to resolve their current IP, since
that isn't cached alongside the paired application key.

By default, the bridge ID and light ID columns are hidden - pass
'-o wide' to include them.

Example:
  homelab lights ls
  homelab lights ls -o wide`,
	RunE: runLs,
}

func init() {
	lsCmd.Flags().StringP("output", "o", "", "Output format. One of: wide")
	addMethodFlag(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
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

	var allLights []hue.Light
	for _, paired := range infraCfg.Lights.Hue.Bridges {
		ip, ok := ipByID[paired.ID]
		if !ok {
			slog.Warn("Paired bridge not found on the network", "id", paired.ID)
			continue
		}

		lights, err := hue.FetchLights(cmd.Context(), ip, paired.ID, paired.AppKey)
		if err != nil {
			slog.Warn("Failed to fetch lights from bridge", "id", paired.ID, "error", err)
			continue
		}
		allLights = append(allLights, lights...)
	}
	sort.Slice(allLights, func(i, j int) bool { return allLights[i].Name < allLights[j].Name })

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if wide {
		fmt.Fprintln(w, "NAME\tON\tBRIGHTNESS\tCOLOR\tCOLOR TEMP\tFIXTURE TYPE\tPRODUCT\tMODEL\tBRIDGE\tID")
	} else {
		fmt.Fprintln(w, "NAME\tON\tBRIGHTNESS\tCOLOR\tCOLOR TEMP\tFIXTURE TYPE\tPRODUCT\tMODEL")
	}

	for _, l := range allLights {
		on := "off"
		if l.On {
			on = "on"
		}
		brightness := dashIfEmpty(fmt.Sprintf("%.0f%%", l.Brightness), l.Brightness < 0)
		color := dashIfEmpty(l.Color, l.Color == "")
		colorTemp := dashIfEmpty(fmt.Sprintf("%dK", l.ColorTempK), l.ColorTempK == 0)
		fixtureType := dashIfEmpty(l.FixtureType, l.FixtureType == "")
		product := dashIfEmpty(l.Product, l.Product == "")
		model := dashIfEmpty(l.Model, l.Model == "")

		if wide {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				l.Name, on, brightness, color, colorTemp, fixtureType, product, model, l.BridgeID, l.ID)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				l.Name, on, brightness, color, colorTemp, fixtureType, product, model)
		}
	}
	return w.Flush()
}

func dashIfEmpty(s string, empty bool) string {
	if empty {
		return "-"
	}
	return s
}
