package flightservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/storage"
)

// newTestService returns a Service with no flightdata.Client configured -
// exercises the "flight sync not configured" path AddFlight/SyncFlight need
// to degrade gracefully through.
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

	return New(storage.NewFlights(db), nil), trip.ID
}

func TestService_AddFlight_RejectsEmptyFields(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	cases := []*v1.AddFlightRequest{
		{TripId: "", FlightNumber: "UA523", Date: "2026-08-01"},
		{TripId: tripID, FlightNumber: "  ", Date: "2026-08-01"},
		{TripId: tripID, FlightNumber: "UA523", Date: ""},
	}
	for _, req := range cases {
		_, err := svc.AddFlight(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("AddFlight(%+v) code = %v, want %v", req, connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	}
}

func TestService_AddFlight_CreatesEvenWithoutSyncClient(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	resp, err := svc.AddFlight(ctx, connect.NewRequest(&v1.AddFlightRequest{
		TripId:       tripID,
		FlightNumber: "ua523",
		Date:         "2026-08-01",
	}))
	if err != nil {
		t.Fatalf("AddFlight() error = %v", err)
	}

	flight := resp.Msg.GetFlight()
	if flight.GetId() == "" {
		t.Fatal("AddFlight() returned empty flight id")
	}
	if flight.GetFlightNumber() != "UA523" {
		t.Fatalf("FlightNumber = %q, want %q (uppercased)", flight.GetFlightNumber(), "UA523")
	}
	if flight.GetStatus() != v1.FlightStatus_FLIGHT_STATUS_UNKNOWN {
		t.Fatalf("Status = %v, want FLIGHT_STATUS_UNKNOWN (no sync client configured)", flight.GetStatus())
	}
}

func TestService_ListFlights_ReturnsAddedFlight(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddFlight(ctx, connect.NewRequest(&v1.AddFlightRequest{
		TripId:       tripID,
		FlightNumber: "UA523",
		Date:         "2026-08-01",
	}))
	if err != nil {
		t.Fatalf("AddFlight() error = %v", err)
	}

	listed, err := svc.ListFlights(ctx, connect.NewRequest(&v1.ListFlightsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListFlights() error = %v", err)
	}
	if len(listed.Msg.GetFlights()) != 1 {
		t.Fatalf("ListFlights() = %d flights, want 1", len(listed.Msg.GetFlights()))
	}
	if listed.Msg.GetFlights()[0].GetId() != added.Msg.GetFlight().GetId() {
		t.Fatalf("ListFlights()[0].Id = %q, want %q", listed.Msg.GetFlights()[0].GetId(), added.Msg.GetFlight().GetId())
	}
}

func TestService_DeleteFlight_ThenListFlights(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddFlight(ctx, connect.NewRequest(&v1.AddFlightRequest{
		TripId:       tripID,
		FlightNumber: "UA523",
		Date:         "2026-08-01",
	}))
	if err != nil {
		t.Fatalf("AddFlight() error = %v", err)
	}

	if _, err := svc.DeleteFlight(ctx, connect.NewRequest(&v1.DeleteFlightRequest{Id: added.Msg.GetFlight().GetId()})); err != nil {
		t.Fatalf("DeleteFlight() error = %v", err)
	}

	listed, err := svc.ListFlights(ctx, connect.NewRequest(&v1.ListFlightsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListFlights() error = %v", err)
	}
	if len(listed.Msg.GetFlights()) != 0 {
		t.Fatalf("ListFlights() after delete = %d flights, want 0", len(listed.Msg.GetFlights()))
	}
}

func TestService_DeleteFlight_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteFlight(context.Background(), connect.NewRequest(&v1.DeleteFlightRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteFlight() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_SyncFlight_FailsPreconditionWithoutClient(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddFlight(ctx, connect.NewRequest(&v1.AddFlightRequest{
		TripId:       tripID,
		FlightNumber: "UA523",
		Date:         "2026-08-01",
	}))
	if err != nil {
		t.Fatalf("AddFlight() error = %v", err)
	}

	_, err = svc.SyncFlight(ctx, connect.NewRequest(&v1.SyncFlightRequest{Id: added.Msg.GetFlight().GetId()}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("SyncFlight() code = %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
	}
}
