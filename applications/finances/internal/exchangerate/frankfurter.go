// Package exchangerate fetches the GBP->USD exchange rate from Frankfurter
// (a free, keyless rate API - https://frankfurter.dev) and keeps
// storage.ExchangeRates populated via a weekly refresh. GetRates
// (exchangerateservice) never calls out live - this is the only thing in
// the app that talks to Frankfurter.
package exchangerate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const ratesURL = "https://api.frankfurter.dev/v2/rates?base=GBP&quotes=USD"

// Client fetches the GBP->USD rate from Frankfurter.
type Client struct {
	httpClient *http.Client
}

// NewClient returns a Client with a reasonable request timeout - this is a
// personal-app background job, not a latency-sensitive request path, so a
// slow or unreachable API just delays that week's rate, not any user
// request.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

type rate struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

// Latest fetches today's GBP->USD rate and the date Frankfurter actually
// has a rate for (the most recent business day, which may be a day or more
// behind on weekends/holidays).
func (c *Client) Latest(ctx context.Context) (usdPerGBP float64, date string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ratesURL, nil)
	if err != nil {
		return 0, "", fmt.Errorf("building frankfurter request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("calling frankfurter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("frankfurter returned status %d", resp.StatusCode)
	}

	var rates []rate
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return 0, "", fmt.Errorf("decoding frankfurter response: %w", err)
	}

	for _, r := range rates {
		if r.Quote == "USD" {
			return r.Rate, r.Date, nil
		}
	}

	return 0, "", fmt.Errorf("frankfurter response missing a GBP->USD rate: %+v", rates)
}
