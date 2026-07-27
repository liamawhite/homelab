// Package flightservice implements the trips.v1.FlightService Connect
// handler over the storage.Flights repository, syncing via
// internal/flightsync when a flightdata.Client is configured.
package flightservice

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/flightdata"
	"github.com/liamawhite/trips/internal/flightsync"
	"github.com/liamawhite/trips/internal/storage"
)

// initialSyncTimeout bounds the best-effort sync AddFlight attempts right
// after creating a flight - long enough for a normal API round trip, short
// enough that a slow/hanging external call can't stall the request
// indefinitely.
const initialSyncTimeout = 15 * time.Second

// Service implements trips.v1.FlightService. client is nil when no
// AeroDataBox API key is configured - see New's doc comment.
type Service struct {
	flights *storage.Flights
	client  *flightdata.Client
}

// New returns a Service backed by flights. client may be nil (no API key
// configured): List/Add/Delete still work, AddFlight simply skips its
// initial sync, and SyncFlight returns CodeFailedPrecondition.
func New(flights *storage.Flights, client *flightdata.Client) *Service {
	return &Service{flights: flights, client: client}
}

// ListFlights returns every flight attached to a trip.
func (s *Service) ListFlights(ctx context.Context, req *connect.Request[v1.ListFlightsRequest]) (*connect.Response[v1.ListFlightsResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}

	flights, err := s.flights.ByTrip(ctx, tripID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListFlightsResponse{Flights: make([]*v1.Flight, 0, len(flights))}
	for _, flight := range flights {
		resp.Flights = append(resp.Flights, toProto(flight))
	}

	return connect.NewResponse(resp), nil
}

// AddFlight creates a flight, then makes one best-effort attempt to sync it
// immediately if a flightdata.Client is configured. A slow or failing sync
// never fails the request - creation and enrichment are separate concerns.
func (s *Service) AddFlight(ctx context.Context, req *connect.Request[v1.AddFlightRequest]) (*connect.Response[v1.AddFlightResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	flightNumber := strings.ToUpper(strings.TrimSpace(req.Msg.GetFlightNumber()))
	date := strings.TrimSpace(req.Msg.GetDate())

	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}
	if flightNumber == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flight_number must not be empty"))
	}
	if date == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("date must not be empty"))
	}

	flight, err := s.flights.Create(ctx, tripID, flightNumber, date)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if s.client != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), initialSyncTimeout)
		defer cancel()

		// A failed initial sync is not fatal to AddFlight - the flight was
		// created either way, and the user can retry via SyncFlight. Either
		// way, flight is updated to reflect what got persisted (fetched
		// data on success, just the recorded sync_error on failure).
		synced, err := flightsync.SyncOne(syncCtx, s.client, s.flights, flight)
		if err != nil {
			log.Printf("initial sync failed for flight %s (%s): %v", flight.ID, flight.FlightNumber, err)
		}
		// synced is only the zero value if SyncOne couldn't even record the
		// failure (e.g. a DB error) - keep the just-created flight in that
		// case rather than returning an empty one.
		if synced.ID != "" {
			flight = synced
		}
	}

	return connect.NewResponse(&v1.AddFlightResponse{Flight: toProto(flight)}), nil
}

// DeleteFlight deletes a flight.
func (s *Service) DeleteFlight(ctx context.Context, req *connect.Request[v1.DeleteFlightRequest]) (*connect.Response[v1.DeleteFlightResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.flights.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrFlightNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteFlightResponse{}), nil
}

// SyncFlight is the manual "Sync now" RPC - unlike AddFlight's best-effort
// initial sync, a failure here is returned to the caller since the user
// explicitly asked for a fresh fetch.
func (s *Service) SyncFlight(ctx context.Context, req *connect.Request[v1.SyncFlightRequest]) (*connect.Response[v1.SyncFlightResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}
	if s.client == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("flight sync is not configured"))
	}

	flight, err := s.flights.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrFlightNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	synced, err := flightsync.SyncOne(ctx, s.client, s.flights, flight)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.SyncFlightResponse{Flight: toProto(synced)}), nil
}

func toProto(flight storage.Flight) *v1.Flight {
	return &v1.Flight{
		Id:                 flight.ID,
		TripId:             flight.TripID,
		FlightNumber:       flight.FlightNumber,
		Date:               flight.FlightDate,
		DepartureAirport:   flight.DepartureAirport,
		ArrivalAirport:     flight.ArrivalAirport,
		DepartureTimezone:  flight.DepartureTimezone,
		ArrivalTimezone:    flight.ArrivalTimezone,
		ScheduledDeparture: timeToProto(flight.ScheduledDeparture),
		ScheduledArrival:   timeToProto(flight.ScheduledArrival),
		ActualDeparture:    timeToProto(flight.ActualDeparture),
		ActualArrival:      timeToProto(flight.ActualArrival),
		Status:             toProtoStatus(flight.Status),
		LastSyncedAt:       timeToProto(flight.LastSyncedAt),
		SyncError:          flight.SyncError,
		CreatedAt:          timestamppb.New(flight.CreatedAt),
	}
}

func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func toProtoStatus(status string) v1.FlightStatus {
	switch status {
	case string(flightdata.StatusScheduled):
		return v1.FlightStatus_FLIGHT_STATUS_SCHEDULED
	case string(flightdata.StatusActive):
		return v1.FlightStatus_FLIGHT_STATUS_ACTIVE
	case string(flightdata.StatusLanded):
		return v1.FlightStatus_FLIGHT_STATUS_LANDED
	case string(flightdata.StatusCancelled):
		return v1.FlightStatus_FLIGHT_STATUS_CANCELLED
	case string(flightdata.StatusDiverted):
		return v1.FlightStatus_FLIGHT_STATUS_DIVERTED
	default:
		return v1.FlightStatus_FLIGHT_STATUS_UNKNOWN
	}
}
