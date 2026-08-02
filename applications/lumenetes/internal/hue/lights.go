package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
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
	DeviceID    string // RID of the owning device resource - needed to rename the light (see RenameDevice)
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
		device := devices[r.Owner.RID]
		lights = append(lights, parseLightResource(r, bridgeID, device))
	}
	return lights, nil
}

// parseLightResource maps one CLIP v2 light resource to a Light, shared by
// FetchLights (which enriches every light with device info in one batch)
// and FetchLight (which doesn't - see its doc comment).
func parseLightResource(r lightResource, bridgeID string, device DeviceInfo) Light {
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

	return Light{
		ID:          r.ID,
		Name:        r.Metadata.Name,
		BridgeID:    bridgeID,
		DeviceID:    r.Owner.RID,
		On:          r.On.On,
		Brightness:  brightness,
		Color:       color,
		ColorTempK:  colorTempK,
		FixtureType: strings.ReplaceAll(r.Metadata.Archetype, "_", " "),
		Product:     device.Product,
		Model:       device.Model,
	}
}

// UpdateLightState is the subset of a light's state that can be enacted
// via the light resource's own PUT endpoint (renaming is not part of this
// - see RenameDevice).
type UpdateLightState struct {
	On         bool
	Brightness float64 // -1 = light doesn't support dimming, omitted from the PUT
	Color      string  // ""  = light doesn't support color, omitted from the PUT
	ColorTempK int     // 0   = light doesn't support color temp, omitted from the PUT
}

type lightPutOn struct {
	On bool `json:"on"`
}

type lightPutDimming struct {
	Brightness float64 `json:"brightness"`
}

type lightPutXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type lightPutColor struct {
	XY lightPutXY `json:"xy"`
}

type lightPutColorTemperature struct {
	Mirek int `json:"mirek"`
}

type lightPutBody struct {
	On               lightPutOn                `json:"on"`
	Dimming          *lightPutDimming          `json:"dimming,omitempty"`
	Color            *lightPutColor            `json:"color,omitempty"`
	ColorTemperature *lightPutColorTemperature `json:"color_temperature,omitempty"`
}

// UpdateLight pushes desired's on/brightness/color/colorTempK to the light
// at lightID via the CLIP v2 API's own PUT endpoint for light resources.
func UpdateLight(ctx context.Context, ip, appKey, lightID string, desired UpdateLightState) error {
	body := lightPutBody{On: lightPutOn{On: desired.On}}

	if desired.Brightness != -1 {
		body.Dimming = &lightPutDimming{Brightness: desired.Brightness}
	}
	if desired.Color != "" {
		x, y, err := hexToXY(desired.Color)
		if err != nil {
			return fmt.Errorf("failed to convert color %q: %w", desired.Color, err)
		}
		body.Color = &lightPutColor{XY: lightPutXY{X: x, Y: y}}
	}
	if desired.ColorTempK != 0 {
		body.ColorTemperature = &lightPutColorTemperature{Mirek: kelvinToMirek(desired.ColorTempK)}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal light update: %w", err)
	}

	url := fmt.Sprintf("https://%s/clip/v2/resource/light/%s", ip, lightID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	req.Header.Set("hue-application-key", appKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hueClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach %s: %w", url, err)
	}
	return checkAPIErrors(resp, url)
}

// apiError is one entry in the CLIP v2 API's "errors" array.
type apiError struct {
	Description string `json:"description"`
}

// apiErrorResponse is the envelope every CLIP v2 API response - success or
// failure - is wrapped in.
type apiErrorResponse struct {
	Errors []apiError `json:"errors"`
}

// checkAPIErrors reads and closes resp.Body, returning an error if the
// CLIP v2 API signaled a failure - either a non-200 HTTP status, or a
// non-empty "errors" array in an otherwise-200 response (the CLIP v2 API
// returns HTTP 200 even for validation failures, e.g. an out-of-range
// brightness, signaling them only via that array).
func checkAPIErrors(resp *http.Response, url string) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d: %s", url, resp.StatusCode, body)
	}
	var parsed apiErrorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("failed to decode response from %s: %w", url, err)
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, len(parsed.Errors))
		for i, e := range parsed.Errors {
			msgs[i] = e.Description
		}
		return fmt.Errorf("%s returned errors: %s", url, strings.Join(msgs, "; "))
	}
	return nil
}
