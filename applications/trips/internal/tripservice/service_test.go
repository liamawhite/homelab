package tripservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/trips/gen/trips/v1"
	"github.com/liamawhite/trips/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	return New(storage.NewTrips(db))
}

func TestService_CreateTrip_RejectsEmptyName(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateTrip(context.Background(), connect.NewRequest(&v1.CreateTripRequest{Name: "   "}))
	if err == nil {
		t.Fatal("CreateTrip() with blank name error = nil, want CodeInvalidArgument")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateTrip() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateTrip_ThenListTrips(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateTrip(ctx, connect.NewRequest(&v1.CreateTripRequest{Name: "Iceland"}))
	if err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}
	if created.Msg.GetTrip().GetId() == "" {
		t.Fatal("CreateTrip() returned empty trip id")
	}
	if created.Msg.GetTrip().GetName() != "Iceland" {
		t.Fatalf("CreateTrip() name = %q, want %q", created.Msg.GetTrip().GetName(), "Iceland")
	}
	if created.Msg.GetTrip().GetCreatedAt() == nil {
		t.Fatal("CreateTrip() returned nil created_at")
	}

	listed, err := svc.ListTrips(ctx, connect.NewRequest(&v1.ListTripsRequest{}))
	if err != nil {
		t.Fatalf("ListTrips() error = %v", err)
	}
	if len(listed.Msg.GetTrips()) != 1 {
		t.Fatalf("ListTrips() = %d trips, want 1", len(listed.Msg.GetTrips()))
	}
	if listed.Msg.GetTrips()[0].GetId() != created.Msg.GetTrip().GetId() {
		t.Fatalf("ListTrips()[0].Id = %q, want %q", listed.Msg.GetTrips()[0].GetId(), created.Msg.GetTrip().GetId())
	}
}

func TestService_DeleteTrip_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteTrip(context.Background(), connect.NewRequest(&v1.DeleteTripRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteTrip() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteTrip_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteTrip(context.Background(), connect.NewRequest(&v1.DeleteTripRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteTrip() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteTrip_ThenListTrips(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateTrip(ctx, connect.NewRequest(&v1.CreateTripRequest{Name: "Iceland"}))
	if err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	if _, err := svc.DeleteTrip(ctx, connect.NewRequest(&v1.DeleteTripRequest{Id: created.Msg.GetTrip().GetId()})); err != nil {
		t.Fatalf("DeleteTrip() error = %v", err)
	}

	listed, err := svc.ListTrips(ctx, connect.NewRequest(&v1.ListTripsRequest{}))
	if err != nil {
		t.Fatalf("ListTrips() error = %v", err)
	}
	if len(listed.Msg.GetTrips()) != 0 {
		t.Fatalf("ListTrips() after delete = %d trips, want 0", len(listed.Msg.GetTrips()))
	}
}
