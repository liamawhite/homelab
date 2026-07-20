package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DeviceInfo is the subset of a bridge device's data useful for display -
// which physical product/model something belongs to, and (for devices
// whose services don't carry their own name, like a switch's buttons) the
// device's own name.
type DeviceInfo struct {
	Name    string
	Product string
	Model   string
}

type deviceResource struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	ProductData struct {
		ModelID     string `json:"model_id"`
		ProductName string `json:"product_name"`
	} `json:"product_data"`
}

type devicesResponse struct {
	Data []deviceResource `json:"data"`
}

// fetchDevices returns every device on the bridge at ip, keyed by device
// ID, so FetchLights can look up the product/model behind each light's
// owner reference without a request per light.
func fetchDevices(ctx context.Context, ip, appKey string) (map[string]DeviceInfo, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/device", ip)
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

	var parsed devicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	devices := make(map[string]DeviceInfo, len(parsed.Data))
	for _, d := range parsed.Data {
		devices[d.ID] = DeviceInfo{Name: d.Metadata.Name, Product: d.ProductData.ProductName, Model: d.ProductData.ModelID}
	}
	return devices, nil
}
