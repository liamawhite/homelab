// Package recurrence computes the next due date for a recurring reminder
// from its rule - kept dependency-free and clock-free (callers pass in
// "today"/"the completion date" explicitly) so the trickiest logic in the
// reminders app is fully deterministic and unit-testable without a database
// or a real clock.
package recurrence

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type RuleType string

const (
	RuleTypeWeekly   RuleType = "weekly"
	RuleTypeMonthly  RuleType = "monthly"
	RuleTypeYearly   RuleType = "yearly"
	RuleTypeInterval RuleType = "interval"
)

// MonthDay is a month/day pair for a yearly rule, e.g. {Month: 7, Day: 4}.
type MonthDay struct {
	Month int
	Day   int
}

// Rule describes when a recurring reminder is due. Exactly the fields
// matching Type are meaningful; the others are ignored.
type Rule struct {
	Type         RuleType
	DaysOfWeek   []time.Weekday // weekly
	DaysOfMonth  []int          // monthly, each 1-31
	YearlyDates  []MonthDay     // yearly
	IntervalDays int            // interval, >= 1
}

// Validate reports whether r is a well-formed rule for its Type.
func (r Rule) Validate() error {
	switch r.Type {
	case RuleTypeWeekly:
		if len(r.DaysOfWeek) == 0 {
			return errors.New("weekly rule requires at least one day of week")
		}
	case RuleTypeMonthly:
		if len(r.DaysOfMonth) == 0 {
			return errors.New("monthly rule requires at least one day of month")
		}
		for _, d := range r.DaysOfMonth {
			if d < 1 || d > 31 {
				return fmt.Errorf("monthly rule day of month %d out of range 1-31", d)
			}
		}
	case RuleTypeYearly:
		if len(r.YearlyDates) == 0 {
			return errors.New("yearly rule requires at least one date")
		}
		for _, md := range r.YearlyDates {
			if md.Month < 1 || md.Month > 12 {
				return fmt.Errorf("yearly rule month %d out of range 1-12", md.Month)
			}
			if md.Day < 1 || md.Day > 31 {
				return fmt.Errorf("yearly rule day %d out of range 1-31", md.Day)
			}
		}
	case RuleTypeInterval:
		if r.IntervalDays < 1 {
			return fmt.Errorf("interval rule interval_days %d must be >= 1", r.IntervalDays)
		}
	default:
		return fmt.Errorf("unknown rule type %q", r.Type)
	}
	return nil
}

// NormalizeDaysOfWeek sorts and deduplicates days, for storage serialization.
func NormalizeDaysOfWeek(days []time.Weekday) []time.Weekday {
	seen := make(map[time.Weekday]bool, len(days))
	out := make([]time.Weekday, 0, len(days))
	for _, d := range days {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NormalizeDaysOfMonth sorts and deduplicates days, for storage serialization.
func NormalizeDaysOfMonth(days []int) []int {
	seen := make(map[int]bool, len(days))
	out := make([]int, 0, len(days))
	for _, d := range days {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Ints(out)
	return out
}

// NormalizeYearlyDates sorts and deduplicates dates, for storage serialization.
func NormalizeYearlyDates(dates []MonthDay) []MonthDay {
	seen := make(map[MonthDay]bool, len(dates))
	out := make([]MonthDay, 0, len(dates))
	for _, d := range dates {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Month != out[j].Month {
			return out[i].Month < out[j].Month
		}
		return out[i].Day < out[j].Day
	})
	return out
}

// TruncateUTC returns UTC midnight of t's calendar day - the convention due
// dates are stored/compared under throughout this app.
func TruncateUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// NextOnCreate returns the first due date for a newly created recurring
// reminder. Calendar rules (weekly/monthly/yearly) match today itself if it
// qualifies; an interval rule is due immediately.
func NextOnCreate(rule Rule, today time.Time) (time.Time, error) {
	if err := rule.Validate(); err != nil {
		return time.Time{}, err
	}

	today = TruncateUTC(today)
	if rule.Type == RuleTypeInterval {
		return today, nil
	}
	return nextCalendarOccurrence(rule, today, true)
}

// NextOnComplete returns the due date that follows marking a recurring
// reminder done. Calendar rules advance from the occurrence that was just
// completed (currentDueDate), so a late completion doesn't shift the
// schedule; an interval rule advances from today (the completion moment),
// so it drifts with however late/early the reminder was actually completed.
func NextOnComplete(rule Rule, currentDueDate, today time.Time) (time.Time, error) {
	if err := rule.Validate(); err != nil {
		return time.Time{}, err
	}

	if rule.Type == RuleTypeInterval {
		return TruncateUTC(today).AddDate(0, 0, rule.IntervalDays), nil
	}
	return nextCalendarOccurrence(rule, TruncateUTC(currentDueDate), false)
}

func nextCalendarOccurrence(rule Rule, anchor time.Time, inclusive bool) (time.Time, error) {
	switch rule.Type {
	case RuleTypeWeekly:
		return nextWeekly(rule.DaysOfWeek, anchor, inclusive), nil
	case RuleTypeMonthly:
		return nextMonthly(rule.DaysOfMonth, anchor, inclusive), nil
	case RuleTypeYearly:
		return nextYearly(rule.YearlyDates, anchor, inclusive), nil
	default:
		return time.Time{}, fmt.Errorf("rule type %q has no calendar occurrence", rule.Type)
	}
}

func nextWeekly(days []time.Weekday, anchor time.Time, inclusive bool) time.Time {
	set := make(map[time.Weekday]bool, len(days))
	for _, d := range days {
		set[d] = true
	}

	start := 0
	if !inclusive {
		start = 1
	}
	// days is non-empty (Validate) so this is guaranteed to find a match
	// within one week of start.
	for o := start; o < start+7; o++ {
		candidate := anchor.AddDate(0, 0, o)
		if set[candidate.Weekday()] {
			return candidate
		}
	}
	panic("recurrence: nextWeekly found no matching weekday")
}

// clampDay returns day clamped to the last valid day of year/month, e.g.
// day=31 in April clamps to 30, day=29 in a non-leap February clamps to 28.
func clampDay(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		return last
	}
	return day
}

func addMonth(year int, month time.Month) (int, time.Month) {
	month++
	if month > 12 {
		return year + 1, 1
	}
	return year, month
}

func nextMonthly(daysOfMonth []int, anchor time.Time, inclusive bool) time.Time {
	year, month := anchor.Year(), anchor.Month()

	// daysOfMonth is non-empty (Validate), so once month/year advances past
	// anchor's own month every candidate qualifies - two iterations suffice,
	// but a slightly larger bound keeps this robust to future refactors.
	for i := 0; i < 24; i++ {
		if candidate, ok := earliestInPeriod(daysOfMonth, func(d int) time.Time {
			return time.Date(year, month, clampDay(year, month, d), 0, 0, 0, 0, time.UTC)
		}, anchor, inclusive); ok {
			return candidate
		}
		year, month = addMonth(year, month)
	}
	panic("recurrence: nextMonthly found no matching day")
}

func nextYearly(dates []MonthDay, anchor time.Time, inclusive bool) time.Time {
	year := anchor.Year()

	for i := 0; i < 8; i++ {
		if candidate, ok := earliestInPeriod(dates, func(md MonthDay) time.Time {
			month := time.Month(md.Month)
			return time.Date(year, month, clampDay(year, month, md.Day), 0, 0, 0, 0, time.UTC)
		}, anchor, inclusive); ok {
			return candidate
		}
		year++
	}
	panic("recurrence: nextYearly found no matching date")
}

// earliestInPeriod returns the earliest of toCandidate(v) over values that
// satisfies the inclusive/exclusive bound against anchor, if any does.
func earliestInPeriod[T any](values []T, toCandidate func(T) time.Time, anchor time.Time, inclusive bool) (time.Time, bool) {
	var best time.Time
	found := false
	for _, v := range values {
		candidate := toCandidate(v)
		if inclusive && candidate.Before(anchor) {
			continue
		}
		if !inclusive && !candidate.After(anchor) {
			continue
		}
		if !found || candidate.Before(best) {
			best = candidate
			found = true
		}
	}
	return best, found
}
