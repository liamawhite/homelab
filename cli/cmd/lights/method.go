package lights

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/lights/hue"
	"github.com/spf13/cobra"
)

// addMethodFlag registers the --bridge-discovery-method flag shared by
// every command that discovers bridges, defaulting to ssdp: it's fully
// local (no dependency on discovery.meethue.com, which rate-limits
// repeated lookups), and reliable once pkg/lights/hue/discover.go's
// timing bug against ssdp.Search's own wait window was fixed.
func addMethodFlag(cmd *cobra.Command) {
	cmd.Flags().String("bridge-discovery-method", string(hue.MethodSSDP), "Bridge discovery method: nupnp, ssdp, or both")
}

func methodFlag(cmd *cobra.Command) (hue.Method, error) {
	raw, err := cmd.Flags().GetString("bridge-discovery-method")
	if err != nil {
		return "", fmt.Errorf("failed to read --bridge-discovery-method flag: %w", err)
	}
	return hue.ParseMethod(raw)
}
