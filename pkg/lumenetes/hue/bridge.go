// Package hue discovers Philips Hue bridges on the local network.
package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Bridge describes a discovered and verified Hue bridge.
type Bridge struct {
	ID         string
	Name       string
	IP         string
	ModelID    string
	APIVersion string
	SWVersion  string
	MAC        string
}

// bridgeConfig is the relevant subset of the unauthenticated GET
// /api/config response every Hue bridge exposes without an API key.
type bridgeConfig struct {
	Name       string `json:"name"`
	BridgeID   string `json:"bridgeid"`
	ModelID    string `json:"modelid"`
	APIVersion string `json:"apiversion"`
	SWVersion  string `json:"swversion"`
	MAC        string `json:"mac"`
}

// FetchInfo queries ip's unauthenticated /api/config endpoint. A non-empty
// bridgeid in the response is what confirms ip is a genuine Hue bridge
// (as opposed to some other UPnP device that responded to an SSDP search).
func FetchInfo(ctx context.Context, ip string) (*Bridge, error) {
	url := fmt.Sprintf("http://%s/api/config", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	var cfg bridgeConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	if cfg.BridgeID == "" {
		return nil, fmt.Errorf("%s did not identify itself as a Hue bridge", ip)
	}

	return &Bridge{
		ID:         cfg.BridgeID,
		Name:       cfg.Name,
		IP:         ip,
		ModelID:    cfg.ModelID,
		APIVersion: cfg.APIVersion,
		SWVersion:  cfg.SWVersion,
		MAC:        cfg.MAC,
	}, nil
}
