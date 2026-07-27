// Package accommodationservice implements the trips.v1.AccommodationService
// Connect handler over the storage.Accommodations repository. Unlike
// flightservice, there's no external source of truth to sync from - every
// field is exactly what the user entered.
package accommodationservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/storage"
)

// Service implements trips.v1.AccommodationService.
type Service struct {
	accommodations *storage.Accommodations
}

// New returns a Service backed by accommodations.
func New(accommodations *storage.Accommodations) *Service {
	return &Service{accommodations: accommodations}
}

// ListAccommodations returns every accommodation attached to a trip.
func (s *Service) ListAccommodations(ctx context.Context, req *connect.Request[v1.ListAccommodationsRequest]) (*connect.Response[v1.ListAccommodationsResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}

	accommodations, err := s.accommodations.ByTrip(ctx, tripID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListAccommodationsResponse{Accommodations: make([]*v1.Accommodation, 0, len(accommodations))}
	for _, accommodation := range accommodations {
		resp.Accommodations = append(resp.Accommodations, toProto(accommodation))
	}

	return connect.NewResponse(resp), nil
}

// AddAccommodation creates a new accommodation.
func (s *Service) AddAccommodation(ctx context.Context, req *connect.Request[v1.AddAccommodationRequest]) (*connect.Response[v1.AddAccommodationResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	name := strings.TrimSpace(req.Msg.GetName())
	location := strings.TrimSpace(req.Msg.GetLocation())
	checkIn := strings.TrimSpace(req.Msg.GetCheckIn())
	checkOut := strings.TrimSpace(req.Msg.GetCheckOut())

	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}
	if checkIn == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("check_in must not be empty"))
	}
	if checkOut == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("check_out must not be empty"))
	}
	if checkOut < checkIn {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("check_out must not be before check_in"))
	}

	accommodation, err := s.accommodations.Create(ctx, tripID, name, location, checkIn, checkOut)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.AddAccommodationResponse{Accommodation: toProto(accommodation)}), nil
}

// DeleteAccommodation deletes an accommodation.
func (s *Service) DeleteAccommodation(ctx context.Context, req *connect.Request[v1.DeleteAccommodationRequest]) (*connect.Response[v1.DeleteAccommodationResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.accommodations.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrAccommodationNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteAccommodationResponse{}), nil
}

func toProto(accommodation storage.Accommodation) *v1.Accommodation {
	return &v1.Accommodation{
		Id:        accommodation.ID,
		TripId:    accommodation.TripID,
		Name:      accommodation.Name,
		Location:  accommodation.Location,
		CheckIn:   accommodation.CheckIn,
		CheckOut:  accommodation.CheckOut,
		CreatedAt: timestamppb.New(accommodation.CreatedAt),
	}
}
