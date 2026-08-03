package itineraryservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/placesdata"
	"github.com/liamawhite/trips/internal/storage"
)

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	trip, err := storage.NewTrips(db).Create(context.Background(), "Iceland")
	if err != nil {
		t.Fatalf("creating test trip: %v", err)
	}

	return New(storage.NewItineraryItems(db), nil), trip.ID
}

// newTestServiceWithPlaces is like newTestService but wires a placesdata
// client pointed at a fake Places server, for AddItineraryItem/
// SearchLocations tests that need location resolution to succeed.
func newTestServiceWithPlaces(t *testing.T, handler http.HandlerFunc) (*Service, string) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	trip, err := storage.NewTrips(db).Create(context.Background(), "Iceland")
	if err != nil {
		t.Fatalf("creating test trip: %v", err)
	}

	return New(storage.NewItineraryItems(db), placesdata.NewClientForTest(server.URL)), trip.ID
}

func TestAddItineraryItem_RejectsEmptyFields(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	cases := []*v1.AddItineraryItemRequest{
		{TripId: "", Title: "Museum", Timezone: "UTC"},
		{TripId: tripID, Title: "  ", Timezone: "UTC"},
		{TripId: tripID, Title: "Museum", Timezone: ""},
		{TripId: tripID, Title: "Museum", Timezone: "Not/AZone"},
	}
	for _, req := range cases {
		_, err := svc.AddItineraryItem(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("AddItineraryItem(%+v) code = %v, want %v", req, connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	}
}

func TestAddItineraryItem_ThenListItineraryItems(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddItineraryItem(ctx, connect.NewRequest(&v1.AddItineraryItemRequest{
		TripId:   tripID,
		Title:    "Visit the museum",
		Timezone: "Europe/London",
	}))
	if err != nil {
		t.Fatalf("AddItineraryItem() error = %v", err)
	}
	if added.Msg.GetItineraryItem().GetId() == "" {
		t.Fatal("AddItineraryItem() returned empty id")
	}
	if added.Msg.GetItineraryItem().GetState() != v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSCHEDULED {
		t.Errorf("State = %v, want UNSCHEDULED", added.Msg.GetItineraryItem().GetState())
	}
	if added.Msg.GetItineraryItem().GetTimezone() != "Europe/London" {
		t.Errorf("Timezone = %q, want %q", added.Msg.GetItineraryItem().GetTimezone(), "Europe/London")
	}

	listed, err := svc.ListItineraryItems(ctx, connect.NewRequest(&v1.ListItineraryItemsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListItineraryItems() error = %v", err)
	}
	if len(listed.Msg.GetItineraryItems()) != 1 {
		t.Fatalf("ListItineraryItems() = %d, want 1", len(listed.Msg.GetItineraryItems()))
	}
}

func TestAddItineraryItem_ResolvesPlaceID(t *testing.T) {
	svc, tripID := newTestServiceWithPlaces(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"formattedAddress": "Eiffel Tower, Paris, France", "location": {"latitude": 48.85, "longitude": 2.29}}`))
	})

	added, err := svc.AddItineraryItem(context.Background(), connect.NewRequest(&v1.AddItineraryItemRequest{
		TripId:   tripID,
		Title:    "See the tower",
		PlaceId:  "some-place-id",
		Timezone: "Europe/Paris",
	}))
	if err != nil {
		t.Fatalf("AddItineraryItem() error = %v", err)
	}
	item := added.Msg.GetItineraryItem()
	if item.GetLocation() != "Eiffel Tower, Paris, France" {
		t.Errorf("Location = %q, want %q", item.GetLocation(), "Eiffel Tower, Paris, France")
	}
	if item.GetLatitude() != 48.85 || item.GetLongitude() != 2.29 {
		t.Errorf("Latitude/Longitude = %v/%v, want 48.85/2.29", item.GetLatitude(), item.GetLongitude())
	}
	if item.GetPlaceId() != "some-place-id" {
		t.Errorf("PlaceId = %q, want %q", item.GetPlaceId(), "some-place-id")
	}
}

func TestAddItineraryItem_PlaceIDWithoutClientConfigured(t *testing.T) {
	svc, tripID := newTestService(t)

	_, err := svc.AddItineraryItem(context.Background(), connect.NewRequest(&v1.AddItineraryItemRequest{
		TripId:   tripID,
		Title:    "See the tower",
		PlaceId:  "some-place-id",
		Timezone: "UTC",
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("AddItineraryItem() code = %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
	}
}

func TestSearchLocations_WithoutClientConfigured(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.SearchLocations(context.Background(), connect.NewRequest(&v1.SearchLocationsRequest{Query: "Eiffel"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("SearchLocations() code = %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
	}
}

func addUnscheduledItem(t *testing.T, svc *Service, tripID string) string {
	t.Helper()
	added, err := svc.AddItineraryItem(context.Background(), connect.NewRequest(&v1.AddItineraryItemRequest{
		TripId:   tripID,
		Title:    "Visit the museum",
		Timezone: "UTC",
	}))
	if err != nil {
		t.Fatalf("AddItineraryItem() error = %v", err)
	}
	return added.Msg.GetItineraryItem().GetId()
}

func TestScheduleItineraryItem_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.ScheduleItineraryItem(context.Background(), connect.NewRequest(&v1.ScheduleItineraryItemRequest{
		Id:       "does-not-exist",
		State:    v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY,
		Date:     "2026-08-01",
		Timezone: "UTC",
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("ScheduleItineraryItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestScheduleItineraryItem_RejectsInvalidPlacements(t *testing.T) {
	svc, tripID := newTestService(t)
	id := addUnscheduledItem(t, svc, tripID)

	cases := []*v1.ScheduleItineraryItemRequest{
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED, Date: "2026-08-01", StartMinute: 120, EndMinute: 60, Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED, Date: "", StartMinute: 60, EndMinute: 120, Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY, Date: "", Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSPECIFIED, Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY, Date: "2026-08-01", Timezone: ""},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY, Date: "2026-08-01", Timezone: "Not/AZone"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY, Date: "2026-08-02", EndDate: "2026-08-01", Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED, Date: "2026-08-02", EndDate: "2026-08-01", StartMinute: 60, EndMinute: 120, Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED, Date: "2026-08-01", StartMinute: -1, EndMinute: 120, Timezone: "UTC"},
		{Id: id, State: v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED, Date: "2026-08-01", StartMinute: 60, EndMinute: 1441, Timezone: "UTC"},
	}
	for _, req := range cases {
		_, err := svc.ScheduleItineraryItem(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("ScheduleItineraryItem(%+v) code = %v, want %v", req, connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	}
}

func TestScheduleItineraryItem_RoundTrip(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()
	id := addUnscheduledItem(t, svc, tripID)

	timed, err := svc.ScheduleItineraryItem(ctx, connect.NewRequest(&v1.ScheduleItineraryItemRequest{
		Id:          id,
		State:       v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED,
		Date:        "2026-08-01",
		StartMinute: 60,
		EndMinute:   120,
		Timezone:    "America/New_York",
	}))
	if err != nil {
		t.Fatalf("ScheduleItineraryItem(TIMED) error = %v", err)
	}
	item := timed.Msg.GetItineraryItem()
	if item.GetState() != v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED || item.GetDate() != "2026-08-01" || item.GetStartMinute() != 60 || item.GetEndMinute() != 120 {
		t.Fatalf("unexpected TIMED item: %+v", item)
	}
	if item.GetTimezone() != "America/New_York" {
		t.Fatalf("Timezone = %q, want %q", item.GetTimezone(), "America/New_York")
	}
	if item.GetEndDate() != "2026-08-01" {
		t.Fatalf("EndDate = %q, want date defaulted to %q", item.GetEndDate(), "2026-08-01")
	}

	unscheduled, err := svc.ScheduleItineraryItem(ctx, connect.NewRequest(&v1.ScheduleItineraryItemRequest{
		Id:          id,
		State:       v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSCHEDULED,
		Date:        "2026-08-01",
		StartMinute: 60,
		EndMinute:   120,
		Timezone:    "America/New_York",
	}))
	if err != nil {
		t.Fatalf("ScheduleItineraryItem(UNSCHEDULED) error = %v", err)
	}
	item = unscheduled.Msg.GetItineraryItem()
	if item.GetState() != v1.ItineraryItemState_ITINERARY_ITEM_STATE_UNSCHEDULED || item.GetDate() != "" || item.GetStartMinute() != 0 || item.GetEndMinute() != 0 || item.GetEndDate() != "" {
		t.Fatalf("unscheduling did not clear date/end_date/minutes, got: %+v", item)
	}
}

// TestScheduleItineraryItem_MultiDaySpan covers the case this feature was
// built for: an item (e.g. a wedding reception) that starts one evening and
// runs past midnight into the next day.
func TestScheduleItineraryItem_MultiDaySpan(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()
	id := addUnscheduledItem(t, svc, tripID)

	spanned, err := svc.ScheduleItineraryItem(ctx, connect.NewRequest(&v1.ScheduleItineraryItemRequest{
		Id:          id,
		State:       v1.ItineraryItemState_ITINERARY_ITEM_STATE_TIMED,
		Date:        "2026-08-07",
		EndDate:     "2026-08-08",
		StartMinute: 13 * 60,
		EndMinute:   60,
		Timezone:    "Europe/London",
	}))
	if err != nil {
		t.Fatalf("ScheduleItineraryItem(multi-day TIMED) error = %v", err)
	}
	item := spanned.Msg.GetItineraryItem()
	if item.GetDate() != "2026-08-07" || item.GetEndDate() != "2026-08-08" || item.GetStartMinute() != 13*60 || item.GetEndMinute() != 60 {
		t.Fatalf("unexpected multi-day TIMED item: %+v", item)
	}

	spannedAllDay, err := svc.ScheduleItineraryItem(ctx, connect.NewRequest(&v1.ScheduleItineraryItemRequest{
		Id:       id,
		State:    v1.ItineraryItemState_ITINERARY_ITEM_STATE_ALL_DAY,
		Date:     "2026-08-07",
		EndDate:  "2026-08-09",
		Timezone: "Europe/London",
	}))
	if err != nil {
		t.Fatalf("ScheduleItineraryItem(multi-day ALL_DAY) error = %v", err)
	}
	item = spannedAllDay.Msg.GetItineraryItem()
	if item.GetDate() != "2026-08-07" || item.GetEndDate() != "2026-08-09" {
		t.Fatalf("unexpected multi-day ALL_DAY item: %+v", item)
	}
}

func TestDeleteItineraryItem_ThenListItineraryItems(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()
	id := addUnscheduledItem(t, svc, tripID)

	if _, err := svc.DeleteItineraryItem(ctx, connect.NewRequest(&v1.DeleteItineraryItemRequest{Id: id})); err != nil {
		t.Fatalf("DeleteItineraryItem() error = %v", err)
	}

	listed, err := svc.ListItineraryItems(ctx, connect.NewRequest(&v1.ListItineraryItemsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListItineraryItems() error = %v", err)
	}
	if len(listed.Msg.GetItineraryItems()) != 0 {
		t.Fatalf("ListItineraryItems() after delete = %d, want 0", len(listed.Msg.GetItineraryItems()))
	}
}

func TestDeleteItineraryItem_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteItineraryItem(context.Background(), connect.NewRequest(&v1.DeleteItineraryItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteItineraryItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}
