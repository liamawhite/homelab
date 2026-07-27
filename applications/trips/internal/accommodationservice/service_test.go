package accommodationservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
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

	return New(storage.NewAccommodations(db)), trip.ID
}

func TestService_AddAccommodation_RejectsEmptyFields(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	cases := []*v1.AddAccommodationRequest{
		{TripId: "", Name: "Hotel", CheckIn: "2026-08-01", CheckOut: "2026-08-05"},
		{TripId: tripID, Name: "  ", CheckIn: "2026-08-01", CheckOut: "2026-08-05"},
		{TripId: tripID, Name: "Hotel", CheckIn: "", CheckOut: "2026-08-05"},
		{TripId: tripID, Name: "Hotel", CheckIn: "2026-08-01", CheckOut: ""},
	}
	for _, req := range cases {
		_, err := svc.AddAccommodation(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("AddAccommodation(%+v) code = %v, want %v", req, connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	}
}

func TestService_AddAccommodation_RejectsCheckOutBeforeCheckIn(t *testing.T) {
	svc, tripID := newTestService(t)

	_, err := svc.AddAccommodation(context.Background(), connect.NewRequest(&v1.AddAccommodationRequest{
		TripId:   tripID,
		Name:     "Hotel",
		CheckIn:  "2026-08-05",
		CheckOut: "2026-08-01",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddAccommodation() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_AddAccommodation_ThenListAccommodations(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddAccommodation(ctx, connect.NewRequest(&v1.AddAccommodationRequest{
		TripId:   tripID,
		Name:     "Premier Inn Farnham",
		Location: "Farnham, UK",
		CheckIn:  "2026-08-05",
		CheckOut: "2026-08-10",
	}))
	if err != nil {
		t.Fatalf("AddAccommodation() error = %v", err)
	}
	if added.Msg.GetAccommodation().GetId() == "" {
		t.Fatal("AddAccommodation() returned empty id")
	}

	listed, err := svc.ListAccommodations(ctx, connect.NewRequest(&v1.ListAccommodationsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListAccommodations() error = %v", err)
	}
	if len(listed.Msg.GetAccommodations()) != 1 {
		t.Fatalf("ListAccommodations() = %d, want 1", len(listed.Msg.GetAccommodations()))
	}
	got := listed.Msg.GetAccommodations()[0]
	if got.GetId() != added.Msg.GetAccommodation().GetId() {
		t.Fatalf("ListAccommodations()[0].Id = %q, want %q", got.GetId(), added.Msg.GetAccommodation().GetId())
	}
	if got.GetLocation() != "Farnham, UK" {
		t.Fatalf("Location = %q, want %q", got.GetLocation(), "Farnham, UK")
	}
}

func TestService_DeleteAccommodation_ThenListAccommodations(t *testing.T) {
	svc, tripID := newTestService(t)
	ctx := context.Background()

	added, err := svc.AddAccommodation(ctx, connect.NewRequest(&v1.AddAccommodationRequest{
		TripId:   tripID,
		Name:     "Hotel",
		CheckIn:  "2026-08-01",
		CheckOut: "2026-08-05",
	}))
	if err != nil {
		t.Fatalf("AddAccommodation() error = %v", err)
	}

	if _, err := svc.DeleteAccommodation(ctx, connect.NewRequest(&v1.DeleteAccommodationRequest{Id: added.Msg.GetAccommodation().GetId()})); err != nil {
		t.Fatalf("DeleteAccommodation() error = %v", err)
	}

	listed, err := svc.ListAccommodations(ctx, connect.NewRequest(&v1.ListAccommodationsRequest{TripId: tripID}))
	if err != nil {
		t.Fatalf("ListAccommodations() error = %v", err)
	}
	if len(listed.Msg.GetAccommodations()) != 0 {
		t.Fatalf("ListAccommodations() after delete = %d, want 0", len(listed.Msg.GetAccommodations()))
	}
}

func TestService_DeleteAccommodation_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteAccommodation(context.Background(), connect.NewRequest(&v1.DeleteAccommodationRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteAccommodation() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}
