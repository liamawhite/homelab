package lights

import (
	"context"
	"fmt"
	"time"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/lights/hue"
	"github.com/spf13/cobra"
)

var hubPairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair with a Hue bridge and save its application key",
	Long: `Registers a new application key with a Hue bridge and saves it to
infra.yaml (lights.hue.bridges) so other lights commands can use it.

The bridge only issues a key within about 30 seconds of its physical link
button being pressed, so this waits and polls until you press it (or
--timeout elapses).

If --ip isn't given, this discovers bridges on the network first and
requires exactly one to be found.

Example:
  homelab lights hub pair
  homelab lights hub pair --ip 192.168.0.165`,
	RunE: runHubPair,
}

func init() {
	hubPairCmd.Flags().String("ip", "", "Bridge IP to pair with (skips discovery; required if more than one bridge is found)")
	hubPairCmd.Flags().Duration("timeout", 60*time.Second, "How long to wait for the link button to be pressed")
	addMethodFlag(hubPairCmd)
}

func runHubPair(cmd *cobra.Command, args []string) error {
	ip, err := cmd.Flags().GetString("ip")
	if err != nil {
		return fmt.Errorf("failed to read --ip flag: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return fmt.Errorf("failed to read --timeout flag: %w", err)
	}
	method, err := methodFlag(cmd)
	if err != nil {
		return err
	}

	configPath, err := config.ResolveConfigPath(cmd)
	if err != nil {
		return err
	}

	if ip == "" {
		bridges, err := hue.Discover(cmd.Context(), 5*time.Second, method)
		if err != nil {
			return err
		}
		switch len(bridges) {
		case 0:
			return fmt.Errorf("no Hue bridges found; pass --ip to target one directly")
		case 1:
			ip = bridges[0].IP
		default:
			return fmt.Errorf("found %d bridges, pass --ip to pick one (run 'homelab lights hub ls' to see them)", len(bridges))
		}
	}

	fmt.Printf("Press the link button on the Hue bridge at %s now - waiting up to %s...\n", ip, timeout)

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	result, err := hue.Pair(ctx, ip, time.Second)
	if err != nil {
		return err
	}

	if err := config.SaveHueBridge(configPath, result.BridgeID, result.AppKey); err != nil {
		return fmt.Errorf("paired successfully but failed to save to %s: %w", configPath, err)
	}

	fmt.Printf("Paired with bridge %s and saved its application key to %s\n", result.BridgeID, configPath)
	return nil
}
