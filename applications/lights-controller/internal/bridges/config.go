// Package bridges loads the paired-bridge id/appKey list the controller
// authenticates to each Hue bridge with. Deliberately NOT
// github.com/liamawhite/homelab/pkg/config.HueBridgeConfig - that package
// imports spf13/cobra and spf13/viper directly, which would pull the CLI's
// whole dependency tree into this binary. The Pulumi component that writes
// the mounted Secret (pkg/components/lightscontroller) and this loader just
// agree on the JSON field names ("id"/"appKey") instead of sharing a type.
package bridges

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config is one paired Hue bridge: its stable bridge ID and the
// application key issued for it via the link-button pairing flow (see
// `homelab lights hub pair`).
type Config struct {
	ID     string `json:"id"`
	AppKey string `json:"appKey"`
}

// ResourceName maps a Hue bridge ID (uppercase hex, e.g.
// "ECB5FAFFFE9D9371", as returned by the bridge's own API and stored
// verbatim in infra.yaml) to the HueBridge CR name that identifies it.
// Kubernetes object names must be a lowercase RFC 1123 subdomain, so both
// hub-controller (which creates/updates HueBridge CRs) and
// lights-controller (which reads them) must derive the same name from the
// same bridge ID - use this everywhere a bridge ID becomes a CR name.
func ResourceName(bridgeID string) string {
	return strings.ToLower(bridgeID)
}

// Load reads and parses the bridges file mounted from the "hue-bridges"
// Secret (see pkg/components/lightscontroller/component.go).
func Load(path string) ([]Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bridges file %s: %w", path, err)
	}

	var cfgs []Config
	if err := json.Unmarshal(data, &cfgs); err != nil {
		return nil, fmt.Errorf("failed to parse bridges file %s: %w", path, err)
	}
	return cfgs, nil
}
