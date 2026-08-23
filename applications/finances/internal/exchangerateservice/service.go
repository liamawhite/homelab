// Package exchangerateservice implements the finances.v1.ExchangeRateService
// Connect handler over the storage.ExchangeRates repository.
package exchangerateservice

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/finances/gen/finances/v1"
	"github.com/liamawhite/finances/internal/storage"
)

// Service implements financesv1connect.ExchangeRateServiceHandler.
type Service struct {
	rates *storage.ExchangeRates
}

// New returns a Service backed by rates.
func New(rates *storage.ExchangeRates) *Service {
	return &Service{rates: rates}
}

// GetRates returns the GBP->USD rate for each requested date, falling back
// to the most recent earlier stored rate for dates with no exact match. A
// date with nothing stored on or before it is simply absent from the
// response - the frontend just can't convert that point yet.
func (s *Service) GetRates(ctx context.Context, req *connect.Request[v1.GetRatesRequest]) (*connect.Response[v1.GetRatesResponse], error) {
	resp := &v1.GetRatesResponse{UsdPerGbpByDate: make(map[string]float64, len(req.Msg.GetDates()))}

	for _, date := range req.Msg.GetDates() {
		rate, err := s.rates.RateOnOrBefore(ctx, date)
		if err != nil {
			if errors.Is(err, storage.ErrExchangeRateNotFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		resp.UsdPerGbpByDate[date] = rate.USDPerGBP
	}

	return connect.NewResponse(resp), nil
}
