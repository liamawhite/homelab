package planner

import (
	v1 "github.com/liamawhite/parental/gen/parental/v1"
	"testing"
	"time"
)

func TestCoverageAndBudgets(t *testing.T) {
	budget := int32(2)
	result := &v1.GetPlanResponse{
		Plan:     &v1.Plan{BirthDate: "2027-01-01"},
		People:   []*v1.Person{{Id: "a", Name: "A", BudgetDays: &budget}, {Id: "b", Name: "B"}},
		Holidays: []*v1.Holiday{{Date: "2027-01-01", Name: "New Year"}, {Date: "2027-01-02", Name: "Weekend holiday"}},
		Assignments: []*v1.Assignment{
			{PersonId: "a", StartDate: "2027-01-01", EndDate: "2027-01-06"},
			{PersonId: "a", StartDate: "2027-01-04", EndDate: "2027-01-05"},
			{PersonId: "b", StartDate: "2027-01-05", EndDate: "2027-01-06"},
		},
	}
	Calculate(result)
	if result.FirstBirthday != "2028-01-01" || len(result.Days) != 387 {
		t.Fatalf("incorrect coverage year: %s, %d", result.FirstBirthday, len(result.Days))
	}
	if result.CoveredDays != 3 || result.MultipleDays != 2 || result.RequiredDays != 260 {
		t.Fatalf("coverage: %+v", result)
	}
	if result.Budgets[0].UsedDays != 3 || result.Budgets[1].UsedDays != 2 {
		t.Fatalf("budgets: %v", result.Budgets)
	}
	if result.Gaps[0].StartDate != "2027-01-07" || result.Gaps[0].EndDate != "2027-12-31" {
		t.Fatalf("gaps: %v", result.Gaps)
	}
	if result.Days[21].Required || result.Days[22].Required || result.Days[23].Required {
		t.Fatal("holidays and weekends must not require coverage")
	}
}

func TestBirthdayBoundaries(t *testing.T) {
	for _, tc := range []struct {
		birth, end string
		days       int
	}{{"2028-02-29", "2029-02-28", 365}, {"2027-03-01", "2028-03-01", 366}, {"2027-12-31", "2028-12-31", 366}} {
		t.Run(tc.birth, func(t *testing.T) {
			result := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: tc.birth}}
			Calculate(result)
			if result.FirstBirthday != tc.end || len(result.Days) != tc.days+22 || result.Days[len(result.Days)-1].Date != tc.end || result.Days[len(result.Days)-1].Required {
				t.Fatalf("wrong birthday: %s, days %d", result.FirstBirthday, len(result.Days))
			}
		})
	}
}

func TestFixedDatesOutsideYearStillUseBudget(t *testing.T) {
	assignments := []*v1.Assignment{{PersonId: "p", StartDate: "2027-01-04", EndDate: "2027-01-08"}, {PersonId: "p", StartDate: "2028-02-01", EndDate: "2028-02-01"}}
	for _, birth := range []string{"2027-01-01", "2027-02-01"} {
		result := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: birth}, People: []*v1.Person{{Id: "p"}}, Assignments: assignments}
		Calculate(result)
		wantCovered := int32(5)
		if birth == "2027-02-01" {
			wantCovered = 0
		}
		if result.CoveredDays != wantCovered || result.Budgets[0].UsedDays != 6 || assignments[0].StartDate != "2027-01-04" {
			t.Fatalf("birth change altered budget or assignments: %+v", result)
		}
	}
}

func TestBudgetUnionMatchesDailyReference(t *testing.T) {
	holidays := map[string]string{"2027-01-01": "New Year", "2027-01-16": "Weekend", "2027-02-01": "Holiday"}
	start, _ := parseDate("2026-12-20")
	for offset := 0; offset < 14; offset++ {
		for length := 0; length < 50; length++ {
			a := []*v1.Assignment{{PersonId: "p", StartDate: start.AddDate(0, 0, offset).Format(dateLayout), EndDate: start.AddDate(0, 0, offset+length).Format(dateLayout)}, {PersonId: "p", StartDate: "2027-01-03", EndDate: "2027-01-20"}}
			var expected int32
			for d := start; d.Before(start.AddDate(0, 0, 80)); d = d.AddDate(0, 0, 1) {
				key := d.Format(dateLayout)
				if !weekday(d) || holidays[key] != "" {
					continue
				}
				for _, r := range a {
					if key >= r.StartDate && key <= r.EndDate {
						expected++
						break
					}
				}
			}
			if actual := usedDays(a, "p", holidays); actual != expected {
				t.Fatalf("offset %d length %d: got %d want %d", offset, length, actual, expected)
			}
		}
	}
}

func TestGapBridgesNonRequiredDays(t *testing.T) {
	result := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: "2027-01-01"}, Holidays: []*v1.Holiday{{Date: "2027-01-04", Name: "Holiday"}}}
	Calculate(result)
	if len(result.Gaps) != 1 || result.Gaps[0].Days != result.RequiredDays {
		t.Fatalf("expected single continuous gap across weekends and holidays")
	}
}

func TestAllCovered(t *testing.T) {
	result := &v1.GetPlanResponse{Plan: &v1.Plan{BirthDate: "2027-01-01"}, People: []*v1.Person{{Id: "p"}}, Assignments: []*v1.Assignment{{PersonId: "p", StartDate: "2027-01-01", EndDate: "2027-12-31"}}}
	Calculate(result)
	if result.CoveredDays != result.RequiredDays || len(result.Gaps) != 0 {
		t.Fatal("full year should have no gaps")
	}
	// Date-only operations remain in UTC, even over DST transitions.
	d, _ := parseDate("2027-03-14")
	if d.Location() != time.UTC {
		t.Fatal("dates must use UTC")
	}
}

func TestOptionalPreBirthAssignments(t *testing.T) {
	assignment := &v1.Assignment{PersonId: "p", StartDate: "2027-12-20", EndDate: "2028-01-09"}
	for _, birth := range []string{"2028-01-10", "2028-01-11"} {
		result := &v1.GetPlanResponse{
			Plan: &v1.Plan{BirthDate: birth}, People: []*v1.Person{{Id: "p"}},
			Assignments: []*v1.Assignment{assignment},
			Holidays:    []*v1.Holiday{{Date: "2027-12-24", Name: "Christmas observed"}, {Date: "2027-12-31", Name: "New Year observed"}},
		}
		Calculate(result)
		birthDate, _ := parseDate(birth)
		if result.Days[0].Date != birthDate.AddDate(0, 0, -21).Format(dateLayout) {
			t.Fatal("planning window must start exactly three weeks before birth")
		}
		for _, day := range result.Days[:21] {
			if day.Required {
				t.Fatal("pre-birth days must be optional")
			}
			d, _ := parseDate(day.Date)
			if weekday(d) && day.Date <= assignment.EndDate && len(day.PersonIds) != 1 {
				t.Fatalf("pre-birth assignment missing on %s", day.Date)
			}
		}
		if result.Budgets[0].UsedDays != 13 || result.CoveredDays != 0 || result.Gaps[0].StartDate != birth {
			t.Fatal("optional assignments must use budget excluding holidays, without affecting required coverage")
		}
		if assignment.StartDate != "2027-12-20" {
			t.Fatal("birth change must preserve assignment dates")
		}
	}
}
