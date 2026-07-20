package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// nupnpURL is Philips' cloud discovery endpoint: every bridge phones home
// on boot and registers its IP here, keyed by the caller's public IP.
const nupnpURL = "https://discovery.meethue.com"

type nupnpEntry struct {
	ID                string `json:"id"`
	InternalIPAddress string `json:"internalipaddress"`
}

// discoverNUPnP returns candidate bridge IPs from Philips' cloud discovery
// endpoint. An empty result is not an error - it just means the endpoint
// has no bridges registered for this caller (e.g. bridge/client on
// different subnets, or the bridge has never had internet access).
func discoverNUPnP(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nupnpURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", nupnpURL, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach %s: %w", nupnpURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", nupnpURL, resp.StatusCode)
	}

	var entries []nupnpEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", nupnpURL, err)
	}

	ips := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.InternalIPAddress != "" {
			ips = append(ips, e.InternalIPAddress)
		}
	}
	return ips, nil
}
