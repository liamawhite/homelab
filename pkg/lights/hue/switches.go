package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Switch describes a single button/control on a physical switch (dimmer
// switch, tap switch, wall switch, etc.), retrieved from a paired
// bridge's v2 CLIP API. Unlike a light, a switch has no ongoing on/off
// state to query - only the most recent event it reported.
type Switch struct {
	ID            string
	BridgeID      string
	Name          string // the owning device's name; buttons have no name of their own
	ControlID     int    // which button/control this is on a multi-button device
	LastEvent     string // e.g. "short_release", "long_press" - empty if never reported
	LastEventTime time.Time
	Battery       int // percentage 0-100; -1 if unknown (e.g. mains-powered)
	Product       string
	Model         string
}

type buttonResource struct {
	ID    string `json:"id"`
	Owner struct {
		RID string `json:"rid"`
	} `json:"owner"`
	Metadata struct {
		ControlID int `json:"control_id"`
	} `json:"metadata"`
	Button struct {
		ButtonReport struct {
			Updated string `json:"updated"`
			Event   string `json:"event"`
		} `json:"button_report"`
	} `json:"button"`
}

type buttonsResponse struct {
	Data []buttonResource `json:"data"`
}

type devicePowerResource struct {
	Owner struct {
		RID string `json:"rid"`
	} `json:"owner"`
	PowerState struct {
		BatteryLevel int `json:"battery_level"`
	} `json:"power_state"`
}

type devicePowerResponse struct {
	Data []devicePowerResource `json:"data"`
}

// FetchSwitches returns every button on the bridge at ip, authenticated
// with appKey, enriched with the owning device's name, product/model, and
// battery level. Failures fetching that enrichment data are tolerated
// (logged as warnings) - it's supplementary to the button data itself.
func FetchSwitches(ctx context.Context, ip, bridgeID, appKey string) ([]Switch, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/button", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	req.Header.Set("hue-application-key", appKey)

	resp, err := hueClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	var parsed buttonsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	devices, err := fetchDevices(ctx, ip, appKey)
	if err != nil {
		slog.Warn("Failed to fetch device info; name/product/model will be blank", "ip", ip, "error", err)
		devices = nil
	}

	battery, err := fetchDevicePower(ctx, ip, appKey)
	if err != nil {
		slog.Warn("Failed to fetch battery info", "ip", ip, "error", err)
		battery = nil
	}

	switches := make([]Switch, 0, len(parsed.Data))
	for _, r := range parsed.Data {
		device := devices[r.Owner.RID]

		batteryLevel := -1
		if level, ok := battery[r.Owner.RID]; ok {
			batteryLevel = level
		}

		// The bridge reports the epoch (1970-01-01) for buttons that have
		// never fired an event, rather than omitting button_report - treat
		// that as "no event yet" rather than a real timestamp.
		var lastEventTime time.Time
		if t, err := time.Parse(time.RFC3339, r.Button.ButtonReport.Updated); err == nil && t.Year() > 1970 {
			lastEventTime = t
		}

		switches = append(switches, Switch{
			ID:            r.ID,
			BridgeID:      bridgeID,
			Name:          device.Name,
			ControlID:     r.Metadata.ControlID,
			LastEvent:     r.Button.ButtonReport.Event,
			LastEventTime: lastEventTime,
			Battery:       batteryLevel,
			Product:       device.Product,
			Model:         device.Model,
		})
	}
	return switches, nil
}

// fetchDevicePower returns every device_power resource's battery level on
// the bridge at ip, keyed by owning device ID.
func fetchDevicePower(ctx context.Context, ip, appKey string) (map[string]int, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/device_power", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	req.Header.Set("hue-application-key", appKey)

	resp, err := hueClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	var parsed devicePowerResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	battery := make(map[string]int, len(parsed.Data))
	for _, d := range parsed.Data {
		battery[d.Owner.RID] = d.PowerState.BatteryLevel
	}
	return battery, nil
}
