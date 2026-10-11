package planner

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/liamawhite/parental/gen/parental/v1"
	"github.com/liamawhite/parental/internal/storage"
)

func TestPersistentPlannerLifecycle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "parental.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatal("migrations must be repeatable", err)
	}
	s := New(db)
	get := func() *v1.GetPlanResponse {
		t.Helper()
		r, e := s.GetPlan(ctx, connect.NewRequest(&v1.GetPlanRequest{}))
		if e != nil {
			t.Fatal(e)
		}
		return r.Msg
	}
	if get().Plan != nil {
		t.Fatal("initial plan should be unset")
	}
	_, err = s.UpdatePlan(ctx, connect.NewRequest(&v1.UpdatePlanRequest{Plan: &v1.Plan{BirthDate: "2027-01-01"}}))
	if err != nil {
		t.Fatal(err)
	}
	budget := int32(1)
	person, err := s.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: &v1.Person{Name: " Parent ", Color: "#377c65", BudgetDays: &budget}}))
	if err != nil {
		t.Fatal(err)
	}
	if person.Msg.Person.Name != "Parent" {
		t.Fatal("name must be trimmed")
	}
	assignment, err := s.SaveAssignment(ctx, connect.NewRequest(&v1.SaveAssignmentRequest{Assignment: &v1.Assignment{PersonId: person.Msg.Person.Id, StartDate: "2027-01-04", EndDate: "2027-01-08"}}))
	if err != nil {
		t.Fatal("over-budget assignment must be accepted", err)
	}
	if get().Budgets[0].UsedDays != 5 {
		t.Fatal("budget calculation incorrect")
	}
	// Reopening the SQLite file simulates the backend restarting.
	db.Close()
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s = New(db)
	if len(get().Assignments) != 1 || get().Plan.BirthDate != "2027-01-01" {
		t.Fatal("plan did not persist")
	}
	a := assignment.Msg.Assignment
	a.EndDate = "2027-01-05"
	if _, err = s.SaveAssignment(ctx, connect.NewRequest(&v1.SaveAssignmentRequest{Assignment: a})); err != nil {
		t.Fatal(err)
	}
	if get().Budgets[0].UsedDays != 2 {
		t.Fatal("editing must update summaries")
	}
	if _, err = s.DeleteAssignment(ctx, connect.NewRequest(&v1.DeleteAssignmentRequest{Id: a.Id})); err != nil {
		t.Fatal(err)
	}
	if get().Budgets[0].UsedDays != 0 {
		t.Fatal("deletion must restore budget")
	}
	a.Id = ""
	if _, err = s.SaveAssignment(ctx, connect.NewRequest(&v1.SaveAssignmentRequest{Assignment: a})); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeletePerson(ctx, connect.NewRequest(&v1.DeletePersonRequest{Id: person.Msg.Person.Id})); err != nil {
		t.Fatal(err)
	}
	if len(get().Assignments) != 0 || len(get().People) != 0 {
		t.Fatal("person deletion must cascade")
	}
}

func TestValidation(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	ctx := context.Background()
	for _, date := range []string{"", "2027-02-29", "2027-1-1", "0000-01-01", "9999-01-01"} {
		_, err := s.UpdatePlan(ctx, connect.NewRequest(&v1.UpdatePlanRequest{Plan: &v1.Plan{BirthDate: date}}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("invalid date %q accepted", date)
		}
	}
	negative := int32(-1)
	for _, p := range []*v1.Person{nil, {Name: " ", Color: "#377c65"}, {Name: "A", Color: "red"}, {Name: "A", Color: "#377c65", BudgetDays: &negative}} {
		_, err := s.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: p}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal("invalid person accepted", p)
		}
	}
	for _, a := range []*v1.Assignment{nil, {PersonId: "missing", StartDate: "2027-01-04", EndDate: "2027-01-05"}, {StartDate: "2027-01-05", EndDate: "2027-01-04"}, {StartDate: "garbage", EndDate: "2027-01-04"}} {
		_, err := s.SaveAssignment(ctx, connect.NewRequest(&v1.SaveAssignmentRequest{Assignment: a}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal("invalid assignment accepted", a)
		}
	}
	_, err = s.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: &v1.Person{Id: "missing", Name: "A", Color: "#377c65"}}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("update must not recreate a deleted person")
	}
}

func TestWashingtonHolidayCalendar(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "holidays.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	ctx := context.Background()
	initial, err := s.GetPlan(ctx, connect.NewRequest(&v1.GetPlanRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Msg.Holidays) != 22 {
		t.Fatalf("expected 22 observed holidays, got %d", len(initial.Msg.Holidays))
	}
	holidays := map[string]string{}
	for _, h := range initial.Msg.Holidays {
		d, err := parseDate(h.Date)
		if err != nil || !weekday(d) {
			t.Fatalf("invalid observed date %s", h.Date)
		}
		holidays[h.Date] = h.Name
	}
	for _, date := range []string{"2027-06-18", "2027-07-05", "2027-11-26", "2027-12-24", "2027-12-31", "2028-11-10", "2028-11-24"} {
		if holidays[date] == "" {
			t.Errorf("missing holiday %s", date)
		}
	}
	for _, date := range []string{"2027-10-11", "2028-10-09"} {
		if holidays[date] != "" {
			t.Errorf("Columbus Day is not a WA holiday: %s", date)
		}
	}
	for _, tc := range []struct {
		birth    string
		required int32
	}{{"2027-01-01", 249}, {"2028-01-01", 250}} {
		r := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: tc.birth}, Holidays: initial.Msg.Holidays}
		Calculate(r)
		if r.RequiredDays != tc.required {
			t.Errorf("%s: required %d, want %d", tc.birth, r.RequiredDays, tc.required)
		}
	}
	// An assignment spanning the observed 2028 New Year uses only Dec 30 and Jan 3.
	r := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: "2027-12-30"}, People: []*v1.Person{{Id: "p"}}, Assignments: []*v1.Assignment{{PersonId: "p", StartDate: "2027-12-30", EndDate: "2028-01-03"}}, Holidays: initial.Msg.Holidays}
	Calculate(r)
	if r.Budgets[0].UsedDays != 2 || r.CoveredDays != 2 {
		t.Fatal("observed cross-year holiday must be excluded from both budget and coverage")
	}
	if r.Days[22].Date != "2027-12-31" || r.Days[22].Required {
		t.Fatal("Dec 31 must not require coverage")
	}
}
