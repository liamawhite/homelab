// Package tripservice implements the trips.v1.TripService Connect handler
// over the storage.Trips repository.
package tripservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/storage"
)

// Service implements tripsv1connect.TripServiceHandler.
type Service struct {
	trips *storage.Trips
}

// New returns a Service backed by trips.
func New(trips *storage.Trips) *Service {
	return &Service{trips: trips}
}

// ListTrips returns every trip.
func (s *Service) ListTrips(ctx context.Context, req *connect.Request[v1.ListTripsRequest]) (*connect.Response[v1.ListTripsResponse], error) {
	trips, err := s.trips.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListTripsResponse{Trips: make([]*v1.Trip, 0, len(trips))}
	for _, trip := range trips {
		resp.Trips = append(resp.Trips, toProto(trip))
	}

	return connect.NewResponse(resp), nil
}

// CreateTrip creates a new trip.
func (s *Service) CreateTrip(ctx context.Context, req *connect.Request[v1.CreateTripRequest]) (*connect.Response[v1.CreateTripResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	trip, err := s.trips.Create(ctx, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateTripResponse{Trip: toProto(trip)}), nil
}

// DeleteTrip deletes a trip.
func (s *Service) DeleteTrip(ctx context.Context, req *connect.Request[v1.DeleteTripRequest]) (*connect.Response[v1.DeleteTripResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.trips.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrTripNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteTripResponse{}), nil
}

func toProto(trip storage.Trip) *v1.Trip {
	return &v1.Trip{
		Id:        trip.ID,
		Name:      trip.Name,
		CreatedAt: timestamppb.New(trip.CreatedAt),
	}
}
