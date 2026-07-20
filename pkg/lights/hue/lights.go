package hue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// Light describes a single light retrieved from a paired bridge's v2 CLIP
// API.
type Light struct {
	ID          string
	Name        string
	BridgeID    string
	On          bool
	Brightness  float64 // percentage 0-100; -1 if the light doesn't support dimming
	Color       string  // approximate "#rrggbb" swatch; "" if the light doesn't support color
	ColorTempK  int     // Kelvin; 0 if the light doesn't support color temperature
	FixtureType string  // e.g. "sultan bulb", "table shade"; "" if unknown
	Product     string  // e.g. "Hue color lamp"; "" if the owning device lookup failed
	Model       string  // e.g. "LCT007"; "" if the owning device lookup failed
}

type lightResource struct {
	ID    string `json:"id"`
	Owner struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"owner"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	On struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
	} `json:"color"`
	ColorTemperature *struct {
		Mirek      int  `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	} `json:"color_temperature"`
}

type lightsResponse struct {
	Data []lightResource `json:"data"`
}

// hueClient skips TLS verification: a bridge's v2 CLIP API is served over
// HTTPS with a self-signed certificate that isn't in any public trust
// store, and there's no practical way to pin it for a LAN-only device
// without vendoring Philips' root CA - every third-party Hue client does
// the same for local bridge connections.
var hueClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
}

// FetchLights returns every light known to the bridge at ip, authenticated
// with appKey (obtained via Pair), enriched with the product/model of
// each light's owning device. A failure to fetch device info is
// tolerated (logged as a warning) - Product/Model are just left blank -
// since it's supplementary to the light data itself.
func FetchLights(ctx context.Context, ip, bridgeID, appKey string) ([]Light, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/light", ip)
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

	var parsed lightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	devices, err := fetchDevices(ctx, ip, appKey)
	if err != nil {
		slog.Warn("Failed to fetch device info; product/model will be blank", "ip", ip, "error", err)
		devices = nil
	}

	lights := make([]Light, 0, len(parsed.Data))
	for _, r := range parsed.Data {
		brightness := -1.0
		if r.Dimming != nil {
			brightness = r.Dimming.Brightness
		}

		var color string
		if r.Color != nil {
			color = xyToHex(r.Color.XY.X, r.Color.XY.Y)
		}

		var colorTempK int
		if r.ColorTemperature != nil && r.ColorTemperature.MirekValid {
			colorTempK = mirekToKelvin(r.ColorTemperature.Mirek)
		}

		device := devices[r.Owner.RID]

		lights = append(lights, Light{
			ID:          r.ID,
			Name:        r.Metadata.Name,
			BridgeID:    bridgeID,
			On:          r.On.On,
			Brightness:  brightness,
			Color:       color,
			ColorTempK:  colorTempK,
			FixtureType: strings.ReplaceAll(r.Metadata.Archetype, "_", " "),
			Product:     device.Product,
			Model:       device.Model,
		})
	}
	return lights, nil
}
