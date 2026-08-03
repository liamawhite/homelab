// Package itineraryservice implements the trips.v1.ItineraryService Connect
// handler over the storage.ItineraryItems repository, resolving locations
// via internal/placesdata when a places.Client is configured.
package itineraryservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	// The trips binary runs FROM scratch (no OS zoneinfo files on disk), so
	// time.LoadLocation below would fail to resolve any zone at all without
	// this - it embeds the full IANA tzdata into the binary at compile time.
	_ "time/tzdata"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/placesdata"
	"github.com/liamawhite/trips/internal/storage"
)

// Service implements trips.v1.ItineraryService. places is nil when no
// Google Places API key is configured - see New's doc comment.
type Service struct {
	items  *storage.ItineraryItems
	places *placesdata.Client
}

// New returns a Service backed by items. places may be nil (no API key
// configured): List/Add(without a location)/Schedule/Delete still work,
// AddItineraryItem rejects a place_id, and SearchLocations returns
// CodeFailedPrecondition.
func New(items *storage.ItineraryItems, places *placesdata.Client) *Service {
	return &Service{items: items, places: places}
}

// ListItineraryItems returns every itinerary item attached to a trip.
func (s *Service) ListItineraryItems(ctx context.Context, req *connect.Request[v1.ListItineraryItemsRequest]) (*connect.Response[v1.ListItineraryItemsResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}

	items, err := s.items.ByTrip(ctx, tripID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListItineraryItemsResponse{ItineraryItems: make([]*v1.ItineraryItem, 0, len(items))}
	for _, item := range items {
		resp.ItineraryItems = append(resp.ItineraryItems, toProto(item))
	}

	return connect.NewResponse(resp), nil
}

// AddItineraryItem creates a new itinerary item, unscheduled. If place_id is
// set, it's resolved server-side via Google Places before the item is
// created - unlike flight sync's best-effort post-creation enrichment, a
// resolution failure here fails the request outright, since the user
// explicitly picked a place before submitting and silently dropping it
// would be confusing rather than a graceful degrade.
func (s *Service) AddItineraryItem(ctx context.Context, req *connect.Request[v1.AddItineraryItemRequest]) (*connect.Response[v1.AddItineraryItemResponse], error) {
	tripID := strings.TrimSpace(req.Msg.GetTripId())
	title := strings.TrimSpace(req.Msg.GetTitle())
	placeID := strings.TrimSpace(req.Msg.GetPlaceId())
	timezone := strings.TrimSpace(req.Msg.GetTimezone())

	if tripID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trip_id must not be empty"))
	}
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title must not be empty"))
	}
	if timezone == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("timezone must not be empty"))
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid timezone %q: %w", timezone, err))
	}

	var location string
	var latitude, longitude float64
	if placeID != "" {
		if s.places == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("location search is not configured"))
		}

		resolved, err := s.places.GetLocation(ctx, placeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		location = resolved.FormattedAddress
		latitude = resolved.Latitude
		longitude = resolved.Longitude
	}

	item, err := s.items.Create(ctx, tripID, title, location, latitude, longitude, placeID, timezone)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.AddItineraryItemResponse{ItineraryItem: toProto(item)}), nil
}

// DeleteItineraryItem deletes an itinerary item.
func (s *Service) DeleteItineraryItem(ctx context.Context, req *connect.Request[v1.DeleteItineraryItemRequest]) (*connect.Response[v1.DeleteItineraryItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.items.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrItineraryItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteItineraryItemResponse{}), nil
}

// ScheduleItineraryItem is the single mutation for every placement change -
// drag-onto-calendar, move, resize, and drag-back-to-backlog (unschedule).
// See itinerary.proto's doc comment on ScheduleItineraryItemRequest for why
// this is one RPC rather than several narrower ones. The service enforces
// the state/date/minute invariant itself rather than trusting the caller,
// since it's the single source of truth for what a valid placement is.
func (s *Service) ScheduleItineraryItem(ctx context.Context, req *connect.Request[v1.ScheduleItineraryItemRequest]) (*connect.Response[v1.ScheduleItineraryItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	timezone := strings.TrimSpace(req.Msg.GetTimezone())
	if timezone == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("timezone must not be empty"))
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid timezone %q: %w", timezone, err))
	}

	date := strings.TrimSpace(req.Msg.GetDate())
	endDate := strings.TrimSpace(req.Msg.GetEndDate())
	startMinute := req.Msg.GetStartMinute()
	endMinute := req.Msg.GetEndMinute()

	var state string
	switch req.Msg.GetState() {
	case v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSCHEDULED:
		state = "UNSCHEDULED"
		date, endDate, startMinute, endMinute = "", "", 0, 0
	case v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY:
		if date == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("date must not be empty for an all-day item"))
		}
		if endDate == "" {
			endDate = date
		}
		if endDate < date {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("end_date must not be before date"))
		}
		state = "ALL_DAY"
		startMinute, endMinute = 0, 0
	case v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED:
		if date == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("date must not be empty for a timed item"))
		}
		if endDate == "" {
			endDate = date
		}
		if endDate < date {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("end_date must not be before date"))
		}
		if startMinute < 0 || startMinute >= 1440 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("start_minute must be within 0-1439"))
		}
		if endMinute < 0 || endMinute > 1440 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("end_minute must be within 0-1440"))
		}
		// A single-day item (end_date == date) still needs end after
		// start - a multi-day item doesn't, since end_minute is on a
		// later calendar day than start_minute regardless of their raw
		// numeric values (e.g. start 13:00 day 1, end 01:00 day 2).
		if endDate == date && startMinute >= endMinute {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("end_minute must be greater than start_minute for a single-day item"))
		}
		state = "TIMED"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("state must be UNSCHEDULED, ALL_DAY, or TIMED"))
	}

	item, err := s.items.Schedule(ctx, id, state, date, endDate, int64(startMinute), int64(endMinute), timezone)
	if err != nil {
		if errors.Is(err, storage.ErrItineraryItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.ScheduleItineraryItemResponse{ItineraryItem: toProto(item)}), nil
}

// SearchLocations proxies a type-ahead location query to Google Places,
// keeping the API key server-side only - the frontend never talks to
// Google directly.
func (s *Service) SearchLocations(ctx context.Context, req *connect.Request[v1.SearchLocationsRequest]) (*connect.Response[v1.SearchLocationsResponse], error) {
	if s.places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("location search is not configured"))
	}

	query := strings.TrimSpace(req.Msg.GetQuery())
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("query must not be empty"))
	}

	candidates, err := s.places.SearchLocations(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.SearchLocationsResponse{Candidates: make([]*v1.LocationCandidate, 0, len(candidates))}
	for _, c := range candidates {
		resp.Candidates = append(resp.Candidates, &v1.LocationCandidate{PlaceId: c.PlaceID, Description: c.Description})
	}

	return connect.NewResponse(resp), nil
}

func toProto(item storage.ItineraryItem) *v1.ItineraryItem {
	return &v1.ItineraryItem{
		Id:          item.ID,
		TripId:      item.TripID,
		Title:       item.Title,
		Location:    item.Location,
		Latitude:    item.Latitude,
		Longitude:   item.Longitude,
		PlaceId:     item.PlaceID,
		State:       toProtoState(item.State),
		Date:        item.Date,
		StartMinute: int32(item.StartMinute),
		EndMinute:   int32(item.EndMinute),
		CreatedAt:   timestamppb.New(item.CreatedAt),
		Timezone:    item.Timezone,
		EndDate:     item.EndDate,
	}
}

func toProtoState(state string) v1.ItineraryItemState {
	switch state {
	case "UNSCHEDULED":
		return v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSCHEDULED
	case "ALL_DAY":
		return v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY
	case "TIMED":
		return v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED
	default:
		return v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSPECIFIED
	}
}
