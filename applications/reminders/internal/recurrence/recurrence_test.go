package recurrence

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestNextOnCreate_Weekly(t *testing.T) {
	rule := Rule{Type: RuleTypeWeekly, DaysOfWeek: []time.Weekday{time.Monday, time.Wednesday, time.Friday}}

	// 2026-08-24 is a Monday - a matching day, should be returned as-is
	// (inclusive).
	got, err := NextOnCreate(rule, date(2026, 8, 24))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 8, 24); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}

	// 2026-08-25 is a Tuesday - the next match is Wednesday the 26th.
	got, err = NextOnCreate(rule, date(2026, 8, 25))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 8, 26); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}

	// 2026-08-29 is a Saturday - wraps into the following week to Monday
	// the 31st.
	got, err = NextOnCreate(rule, date(2026, 8, 29))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 8, 31); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}
}

func TestNextOnComplete_Weekly_ExcludesCurrentDueDate(t *testing.T) {
	rule := Rule{Type: RuleTypeWeekly, DaysOfWeek: []time.Weekday{time.Monday, time.Friday}}

	// Completing the Monday occurrence should advance to Friday, not stay
	// on Monday, even though today is still Monday.
	got, err := NextOnComplete(rule, date(2026, 8, 24), date(2026, 8, 24))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2026, 8, 28); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}

	// Completing the Friday occurrence wraps around to the following
	// Monday.
	got, err = NextOnComplete(rule, date(2026, 8, 28), date(2026, 8, 28))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2026, 8, 31); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}
}

func TestNextOnComplete_Weekly_LateCompletionDoesNotDrift(t *testing.T) {
	rule := Rule{Type: RuleTypeWeekly, DaysOfWeek: []time.Weekday{time.Monday}}

	// Due Monday the 24th, but not actually completed until Thursday the
	// 27th - the next occurrence is still anchored off the due date
	// (the following Monday), not off the completion date.
	got, err := NextOnComplete(rule, date(2026, 8, 24), date(2026, 8, 27))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2026, 8, 31); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}
}

func TestNextOnCreate_Monthly_MultipleDays(t *testing.T) {
	rule := Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{1, 15}}

	// From the 10th, the nearer of the two candidates (1st, 15th) is the
	// 15th.
	got, err := NextOnCreate(rule, date(2026, 8, 10))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 8, 15); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}

	// From the 20th, both candidates in August have passed - rolls into
	// September to the 1st.
	got, err = NextOnCreate(rule, date(2026, 8, 20))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 9, 1); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}
}

func TestNextOnCreate_Monthly_ClampsDayOfMonth(t *testing.T) {
	rule := Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{31}}

	// April only has 30 days - day 31 clamps to April 30th.
	got, err := NextOnCreate(rule, date(2026, 4, 1))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 4, 30); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}
}

func TestNextOnComplete_Monthly_ClampedRollover(t *testing.T) {
	rule := Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{31}}

	// Completing the clamped April 30th occurrence rolls to May, where day
	// 31 is valid and no longer clamped.
	got, err := NextOnComplete(rule, date(2026, 4, 30), date(2026, 4, 30))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2026, 5, 31); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}
}

func TestYearly_Feb29ClampingRoundTrip(t *testing.T) {
	rule := Rule{Type: RuleTypeYearly, YearlyDates: []MonthDay{{Month: 2, Day: 29}}}

	// 2028 is a leap year - Feb 29 exists.
	got, err := NextOnCreate(rule, date(2028, 1, 1))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2028, 2, 29); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}

	// Completing the 2028 leap-day occurrence rolls to 2029, a non-leap
	// year, where Feb 29 clamps to Feb 28.
	got, err = NextOnComplete(rule, date(2028, 2, 29), date(2028, 2, 29))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2029, 2, 28); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}

	// Completing the clamped 2029 occurrence rolls to 2030, still
	// non-leap, still clamped to Feb 28.
	got, err = NextOnComplete(rule, date(2029, 2, 28), date(2029, 2, 28))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2030, 2, 28); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}
}

func TestYearly_MultipleDates(t *testing.T) {
	rule := Rule{Type: RuleTypeYearly, YearlyDates: []MonthDay{{Month: 1, Day: 1}, {Month: 7, Day: 4}}}

	// From March, the nearer candidate is July 4th of the same year.
	got, err := NextOnCreate(rule, date(2026, 3, 1))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 7, 4); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}

	// From August, both 2026 candidates have passed - rolls to Jan 1 2027.
	got, err = NextOnCreate(rule, date(2026, 8, 1))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2027, 1, 1); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}
}

func TestInterval_NextOnCreate_ReturnsToday(t *testing.T) {
	rule := Rule{Type: RuleTypeInterval, IntervalDays: 3}

	got, err := NextOnCreate(rule, date(2026, 8, 24))
	if err != nil {
		t.Fatalf("NextOnCreate() error = %v", err)
	}
	if want := date(2026, 8, 24); !got.Equal(want) {
		t.Fatalf("NextOnCreate() = %v, want %v", got, want)
	}
}

func TestInterval_NextOnComplete_DriftsFromCompletionDate(t *testing.T) {
	rule := Rule{Type: RuleTypeInterval, IntervalDays: 3}

	// Due the 24th, completed late on the 27th - the next occurrence is 3
	// days after the *completion* date (the 30th), not 3 days after the
	// original due date (which would be the 27th).
	got, err := NextOnComplete(rule, date(2026, 8, 24), date(2026, 8, 27))
	if err != nil {
		t.Fatalf("NextOnComplete() error = %v", err)
	}
	if want := date(2026, 8, 30); !got.Equal(want) {
		t.Fatalf("NextOnComplete() = %v, want %v", got, want)
	}
}

func TestRule_Validate(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		wantErr bool
	}{
		{"weekly ok", Rule{Type: RuleTypeWeekly, DaysOfWeek: []time.Weekday{time.Monday}}, false},
		{"weekly empty", Rule{Type: RuleTypeWeekly}, true},
		{"monthly ok", Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{1}}, false},
		{"monthly empty", Rule{Type: RuleTypeMonthly}, true},
		{"monthly out of range", Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{32}}, true},
		{"monthly zero", Rule{Type: RuleTypeMonthly, DaysOfMonth: []int{0}}, true},
		{"yearly ok", Rule{Type: RuleTypeYearly, YearlyDates: []MonthDay{{Month: 1, Day: 1}}}, false},
		{"yearly empty", Rule{Type: RuleTypeYearly}, true},
		{"yearly month out of range", Rule{Type: RuleTypeYearly, YearlyDates: []MonthDay{{Month: 13, Day: 1}}}, true},
		{"yearly day out of range", Rule{Type: RuleTypeYearly, YearlyDates: []MonthDay{{Month: 1, Day: 32}}}, true},
		{"interval ok", Rule{Type: RuleTypeInterval, IntervalDays: 1}, false},
		{"interval zero", Rule{Type: RuleTypeInterval, IntervalDays: 0}, true},
		{"interval negative", Rule{Type: RuleTypeInterval, IntervalDays: -1}, true},
		{"unknown type", Rule{Type: "bogus"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeDaysOfWeek(t *testing.T) {
	got := NormalizeDaysOfWeek([]time.Weekday{time.Friday, time.Monday, time.Monday, time.Wednesday})
	want := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	if len(got) != len(want) {
		t.Fatalf("NormalizeDaysOfWeek() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeDaysOfWeek() = %v, want %v", got, want)
		}
	}
}

func TestNormalizeDaysOfMonth(t *testing.T) {
	got := NormalizeDaysOfMonth([]int{15, 1, 15, 8})
	want := []int{1, 8, 15}
	if len(got) != len(want) {
		t.Fatalf("NormalizeDaysOfMonth() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeDaysOfMonth() = %v, want %v", got, want)
		}
	}
}

func TestNormalizeYearlyDates(t *testing.T) {
	got := NormalizeYearlyDates([]MonthDay{{7, 4}, {1, 1}, {7, 4}, {1, 15}})
	want := []MonthDay{{1, 1}, {1, 15}, {7, 4}}
	if len(got) != len(want) {
		t.Fatalf("NormalizeYearlyDates() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeYearlyDates() = %v, want %v", got, want)
		}
	}
}
