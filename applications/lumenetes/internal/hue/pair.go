package hue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// errLinkButtonNotPressed is requestAppKey's sentinel for Hue error type
// 101 ("link button not pressed") - the signal that Pair should keep
// polling rather than give up.
var errLinkButtonNotPressed = errors.New("link button not pressed")

type pairResponseEntry struct {
	Success *struct {
		Username string `json:"username"`
	} `json:"success"`
	Error *struct {
		Type        int    `json:"type"`
		Description string `json:"description"`
	} `json:"error"`
}

// requestAppKey asks the bridge at ip to issue a new application key. The
// bridge only grants one within roughly 30 seconds of its physical link
// button being pressed; otherwise it responds with error type 101, which
// requestAppKey surfaces as errLinkButtonNotPressed.
func requestAppKey(ctx context.Context, ip string) (string, error) {
	url := fmt.Sprintf("http://%s/api", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(`{"devicetype":"homelab#cli"}`))
	if err != nil {
		return "", fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	var entries []pairResponseEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return "", fmt.Errorf("failed to decode response from %s: %w", url, err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("%s returned an empty response", url)
	}

	switch entry := entries[0]; {
	case entry.Success != nil:
		return entry.Success.Username, nil
	case entry.Error != nil && entry.Error.Type == 101:
		return "", errLinkButtonNotPressed
	case entry.Error != nil:
		return "", fmt.Errorf("bridge returned error %d: %s", entry.Error.Type, entry.Error.Description)
	default:
		return "", fmt.Errorf("unrecognized response from %s", url)
	}
}

// PairResult is the outcome of a successful Pair call.
type PairResult struct {
	BridgeID string
	AppKey   string
}

// Pair registers a new application key with the bridge at ip, polling
// every pollInterval until the link button is pressed or ctx is done
// (typically because the caller bounded it with a timeout).
func Pair(ctx context.Context, ip string, pollInterval time.Duration) (*PairResult, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		appKey, err := requestAppKey(ctx, ip)
		switch {
		case err == nil:
			info, err := FetchInfo(ctx, ip)
			if err != nil {
				return nil, fmt.Errorf("paired successfully but failed to fetch bridge info: %w", err)
			}
			return &PairResult{BridgeID: info.ID, AppKey: appKey}, nil
		case !errors.Is(err, errLinkButtonNotPressed):
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for the link button to be pressed: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
