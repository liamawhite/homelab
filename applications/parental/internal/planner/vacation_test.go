package planner

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/liamawhite/parental/gen/parental/v1"
	"github.com/liamawhite/parental/internal/storage"
)

func TestVacationLifecyclePreservesAccounting(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plan.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	if _, err := s.UpdatePlan(ctx, connect.NewRequest(&v1.UpdatePlanRequest{Plan: &v1.Plan{BirthDate: "2027-01-01"}})); err != nil {
		t.Fatal(err)
	}
	person, err := s.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: &v1.Person{Name: "Parent", Color: "#377c65"}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveAssignment(ctx, connect.NewRequest(&v1.SaveAssignmentRequest{Assignment: &v1.Assignment{PersonId: person.Msg.Person.Id, StartDate: "2027-01-04", EndDate: "2027-01-08"}})); err != nil {
		t.Fatal(err)
	}
	get := func() *v1.GetPlanResponse {
		t.Helper()
		r, e := s.GetPlan(ctx, connect.NewRequest(&v1.GetPlanRequest{}))
		if e != nil {
			t.Fatal(e)
		}
		return r.Msg
	}
	before := get()
	saved, err := s.SaveVacation(ctx, connect.NewRequest(&v1.SaveVacationRequest{Vacation: &v1.Vacation{Name: " Trip ", StartDate: "2027-01-01", EndDate: "2027-01-10"}}))
	if err != nil {
		t.Fatal(err)
	}
	v := saved.Msg.Vacation
	if v.Id == "" || v.Name != "Trip" {
		t.Fatal("vacation must have ID and trimmed name")
	}
	after := get()
	if after.CoveredDays != before.CoveredDays || after.RequiredDays != before.RequiredDays || after.Budgets[0].UsedDays != before.Budgets[0].UsedDays {
		t.Fatal("vacation must not affect accounting")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s = New(db)
	if len(get().Vacations) != 1 || get().Vacations[0].Id != v.Id {
		t.Fatal("vacation must survive reopening database")
	}
	v.Name = "Updated trip"
	v.EndDate = "2027-01-12"
	if _, err := s.SaveVacation(ctx, connect.NewRequest(&v1.SaveVacationRequest{Vacation: v})); err != nil {
		t.Fatal(err)
	}
	if get().Vacations[0].EndDate != v.EndDate {
		t.Fatal("update missing")
	}
	if _, err := s.DeleteVacation(ctx, connect.NewRequest(&v1.DeleteVacationRequest{Id: v.Id})); err != nil {
		t.Fatal(err)
	}
	after = get()
	if len(after.Vacations) != 0 || len(after.Assignments) != 1 || after.Budgets[0].UsedDays != 5 {
		t.Fatal("deleting vacation must preserve assignments")
	}
	if _, err := s.SaveVacation(ctx, connect.NewRequest(&v1.SaveVacationRequest{Vacation: v})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("stale update must not recreate vacation")
	}
	for _, bad := range []*v1.Vacation{nil, {Name: " ", StartDate: "2027-01-01", EndDate: "2027-01-02"}, {Name: "Trip", StartDate: "bad", EndDate: "2027-01-02"}, {Name: "Trip", StartDate: "2027-01-03", EndDate: "2027-01-02"}} {
		if _, err := s.SaveVacation(ctx, connect.NewRequest(&v1.SaveVacationRequest{Vacation: bad})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("expected invalid argument, got %v", err)
		}
	}
}
