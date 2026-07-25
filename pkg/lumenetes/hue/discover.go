package hue

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// enrichTimeout bounds the verification/enrichment phase (one HTTP GET per
// candidate IP). It's a separate, fixed budget from the caller-supplied
// discovery timeout: SSDP's M-SEARCH intentionally blocks for the entire
// discovery timeout window collecting responses, so that window is already
// spent by the time enrichment starts - it needs its own fresh allowance
// rather than sharing what's left of the discovery deadline (which, for a
// short --timeout, could be nothing).
const enrichTimeout = 5 * time.Second

// Method selects which bridge-discovery mechanism(s) Discover uses.
type Method string

const (
	MethodNUPnP Method = "nupnp"
	MethodSSDP  Method = "ssdp"
	MethodBoth  Method = "both"
)

// ParseMethod validates a --bridge-discovery-method flag value.
func ParseMethod(s string) (Method, error) {
	switch m := Method(s); m {
	case MethodNUPnP, MethodSSDP, MethodBoth:
		return m, nil
	default:
		return "", fmt.Errorf("invalid discovery method %q: must be one of %s, %s, %s", s, MethodNUPnP, MethodSSDP, MethodBoth)
	}
}

// Discover finds Hue bridges on the local network using method - N-UPnP
// (cloud), SSDP (local multicast), or both at once. Philips recommends
// running both simultaneously rather than sequentially when using both,
// since back-to-back SSDP lookups can suppress a bridge's second response.
// When method is MethodBoth, either one failing outright is tolerated
// (logged as a warning) as long as the other succeeds; an error is only
// returned if both fail (or the sole selected method fails). Every
// candidate IP is then verified and enriched via FetchInfo, which also
// filters out any non-Hue UPnP devices SSDP may have picked up.
//
// timeout bounds only the discovery phase; enrichment gets its own
// separate allowance (see enrichTimeout). ctx bounds the whole operation
// for cancellation (e.g. Ctrl-C) but should not itself carry a deadline -
// Discover manages timing internally.
//
// Zero bridges found - whether because none exist or every candidate
// failed verification - is not an error: it returns (nil, nil).
func Discover(ctx context.Context, timeout time.Duration, method Method) ([]Bridge, error) {
	discoverCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	useNUPnP := method == MethodNUPnP || method == MethodBoth
	useSSDP := method == MethodSSDP || method == MethodBoth

	var (
		wg                sync.WaitGroup
		nupnpIPs, ssdpIPs []string
		nupnpErr, ssdpErr error
	)

	if useNUPnP {
		wg.Go(func() {
			nupnpIPs, nupnpErr = discoverNUPnP(discoverCtx)
		})
	}
	if useSSDP {
		wg.Go(func() {
			ssdpIPs, ssdpErr = runSSDPWithContext(discoverCtx, timeout)
		})
	}
	wg.Wait()

	if useNUPnP && nupnpErr != nil {
		slog.Warn("N-UPnP discovery failed", "error", nupnpErr)
	}
	if useSSDP && ssdpErr != nil {
		slog.Warn("SSDP discovery failed", "error", ssdpErr)
	}
	switch {
	case useNUPnP && useSSDP && nupnpErr != nil && ssdpErr != nil:
		return nil, fmt.Errorf("both discovery methods failed: nupnp: %w; ssdp: %v", nupnpErr, ssdpErr)
	case useNUPnP && !useSSDP && nupnpErr != nil:
		return nil, nupnpErr
	case useSSDP && !useNUPnP && ssdpErr != nil:
		return nil, ssdpErr
	}

	candidates := dedupe(append(nupnpIPs, ssdpIPs...))

	enrichCtx, enrichCancel := context.WithTimeout(ctx, enrichTimeout)
	defer enrichCancel()

	var (
		mu       sync.Mutex
		bridges  []Bridge
		enrichWg sync.WaitGroup
	)
	for _, ip := range candidates {
		enrichWg.Add(1)
		go func(ip string) {
			defer enrichWg.Done()
			b, err := FetchInfo(enrichCtx, ip)
			if err != nil {
				slog.Warn("Candidate did not verify as a Hue bridge", "ip", ip, "error", err)
				return
			}
			mu.Lock()
			bridges = append(bridges, *b)
			mu.Unlock()
		}(ip)
	}
	enrichWg.Wait()

	sort.Slice(bridges, func(i, j int) bool { return bridges[i].ID < bridges[j].ID })
	return bridges, nil
}

// ssdpBuffer is subtracted from the caller's timeout before sizing SSDP's
// own internal wait window. ssdp.Search always blocks for that entire
// window before returning - by design, it keeps collecting responses from
// multiple devices until the window closes rather than returning as soon
// as it gets one - so its internal deadline and ctx's deadline below
// would otherwise expire within milliseconds of each other (ctx starts
// ticking first, before the search goroutine even starts), and ctx would
// almost always win the race, discarding an already-successful result.
// This buffer gives the search a safety margin to finish and return
// normally; ctx.Done() remains a backstop for a genuinely hung search.
const ssdpBuffer = 750 * time.Millisecond

// runSSDPWithContext runs discoverSSDP (which has no native cancellation)
// in a goroutine and races it against ctx, since the caller's timeout
// should bound the whole discovery operation including SSDP's wait.
func runSSDPWithContext(ctx context.Context, timeout time.Duration) ([]string, error) {
	waitSec := max(int((timeout - ssdpBuffer).Seconds()), 1)

	type result struct {
		ips []string
		err error
	}
	done := make(chan result, 1)
	go func() {
		ips, err := discoverSSDP(waitSec)
		done <- result{ips, err}
	}()

	select {
	case r := <-done:
		return r.ips, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func dedupe(ips []string) []string {
	seen := make(map[string]bool, len(ips))
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}
