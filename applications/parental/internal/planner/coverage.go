package planner

import (
	"sort"
	"time"

	v1 "github.com/liamawhite/parental/gen/parental/v1"
)

const dateLayout = "2006-01-02"

func parseDate(value string) (time.Time, error) { return time.Parse(dateLayout, value) }
func weekday(d time.Time) bool                  { return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday }
func birthday(birth time.Time) time.Time {
	if birth.Month() == time.February && birth.Day() == 29 {
		return time.Date(birth.Year()+1, time.February, 28, 0, 0, 0, 0, time.UTC)
	}
	return birth.AddDate(1, 0, 0)
}

type interval struct{ start, end time.Time }

// Count the union of each person's assignment intervals, including dates outside
// the displayed planning window. Whole weeks are counted arithmetically, avoiding a loop
// over potentially long ranges. Holidays never consume leave.
func usedDays(assignments []*v1.Assignment, personID string, holidays map[string]string) int32 {
	var ranges []interval
	for _, a := range assignments {
		if a.PersonId == personID {
			start, _ := parseDate(a.StartDate)
			end, _ := parseDate(a.EndDate)
			ranges = append(ranges, interval{start, end})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start.Before(ranges[j].start) })
	var merged []interval
	for _, r := range ranges {
		if len(merged) == 0 || r.start.After(merged[len(merged)-1].end.AddDate(0, 0, 1)) {
			merged = append(merged, r)
		} else if r.end.After(merged[len(merged)-1].end) {
			merged[len(merged)-1].end = r.end
		}
	}
	var total int32
	for _, r := range merged {
		days := (r.end.Unix()-r.start.Unix())/86400 + 1
		total += int32(days / 7 * 5)
		for i := int64(0); i < days%7; i++ {
			if weekday(r.start.AddDate(0, 0, int(i))) {
				total++
			}
		}
		for date := range holidays {
			d, _ := parseDate(date)
			if weekday(d) && !d.Before(r.start) && !d.After(r.end) {
				total--
			}
		}
	}
	return total
}

// Calculate is the single source of truth for coverage and budget summaries.
func Calculate(result *v1.GetPlanResponse) {
	holidays := map[string]string{}
	for _, h := range result.Holidays {
		holidays[h.Date] = h.Name
	}
	for _, p := range result.People {
		result.Budgets = append(result.Budgets, &v1.Budget{PersonId: p.Id, UsedDays: usedDays(result.Assignments, p.Id, holidays)})
	}
	if result.Plan == nil {
		return
	}
	start, _ := parseDate(result.Plan.BirthDate)
	end := birthday(start)
	result.FirstBirthday = end.Format(dateLayout)
	var gap *v1.Gap
	for d := start.AddDate(0, 0, -21); !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format(dateLayout)
		required := !d.Before(start) && d.Before(end) && weekday(d) && holidays[key] == ""
		day := &v1.Day{Date: key, Required: required}
		if weekday(d) {
			seen := map[string]bool{}
			for _, a := range result.Assignments {
				if a.StartDate <= key && a.EndDate >= key {
					seen[a.PersonId] = true
				}
			}
			for _, p := range result.People {
				if seen[p.Id] {
					day.PersonIds = append(day.PersonIds, p.Id)
				}
			}
		}
		result.Days = append(result.Days, day)
		if !required {
			continue
		}
		result.RequiredDays++
		if len(day.PersonIds) > 0 {
			result.CoveredDays++
			gap = nil
			if len(day.PersonIds) > 1 {
				result.MultipleDays++
			}
		} else {
			if gap == nil {
				gap = &v1.Gap{StartDate: key}
				result.Gaps = append(result.Gaps, gap)
			}
			gap.EndDate = key
			gap.Days++
		}
	}
}
