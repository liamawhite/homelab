package hue

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/koron/go-ssdp"
)

// discoverSSDP runs a local SSDP M-SEARCH and returns candidate bridge
// IPs, filtered to responses whose SERVER header identifies them as a Hue
// bridge (e.g. "FreeRTOS/6.0.5, UPnP/1.0, IpBridge/0.1"). Hue doesn't
// advertise a distinctive search target, so this searches broadly
// (ssdp.All) and relies on that header - and, downstream, on FetchInfo -
// to filter out unrelated UPnP devices that also respond.
//
// ssdp.Search blocks synchronously for waitSec seconds and takes no
// context.Context, so callers that need cancellation should run this in a
// goroutine and race it against their own context.
func discoverSSDP(waitSec int) ([]string, error) {
	services, err := ssdp.Search(ssdp.All, waitSec, "")
	if err != nil {
		return nil, fmt.Errorf("ssdp search failed: %w", err)
	}

	var ips []string
	for _, svc := range services {
		if !strings.Contains(svc.Server, "IpBridge") {
			continue
		}
		loc, err := url.Parse(svc.Location)
		if err != nil {
			continue
		}
		if host := loc.Hostname(); host != "" {
			ips = append(ips, host)
		}
	}
	return ips, nil
}
