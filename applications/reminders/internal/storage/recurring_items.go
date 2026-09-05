package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/reminders/internal/recurrence"
	"github.com/liamawhite/reminders/internal/storage/sqlcgen"
)

// ErrRecurringItemNotFound is returned by Update*/Complete/Delete when no
// item matches the given ID.
var ErrRecurringItemNotFound = errors.New("recurring item not found")

// RecurringItem is a row from the recurring_items table.
type RecurringItem struct {
	ID          string
	Title       string
	Notes       string
	CreatedAt   time.Time
	NextDueDate time.Time
	Area        Area
	Rule        recurrence.Rule
}

// RecurringItems is the repository for the recurring_items table, backed by
// sqlc-generated queries. Like OneOffItems, it adapts sqlcgen's raw string
// columns to Go-native types on the way out, and generates IDs/timestamps
// and computes next_due_date (via internal/recurrence) on the way in.
type RecurringItems struct {
	q *sqlcgen.Queries
}

// NewRecurringItems returns a RecurringItems repository backed by db.
func NewRecurringItems(db *sql.DB) *RecurringItems {
	return &RecurringItems{q: sqlcgen.New(db)}
}

// List returns every recurring item, soonest due first.
func (i *RecurringItems) List(ctx context.Context) ([]RecurringItem, error) {
	rows, err := i.q.ListRecurringItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing recurring items: %w", err)
	}

	items := make([]RecurringItem, 0, len(rows))
	for _, row := range rows {
		item, err := recurringItemFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Create inserts a new recurring item with a generated ID, computing its
// first due date from rule via recurrence.NextOnCreate(rule, now).
func (i *RecurringItems) Create(ctx context.Context, title, notes string, area Area, rule recurrence.Rule, now time.Time) (RecurringItem, error) {
	nextDueDate, err := recurrence.NextOnCreate(rule, now)
	if err != nil {
		return RecurringItem{}, fmt.Errorf("computing next due date: %w", err)
	}

	cols := ruleColumns(rule)
	row, err := i.q.CreateRecurringItem(ctx, sqlcgen.CreateRecurringItemParams{
		ID:           uuid.NewString(),
		Title:        title,
		Notes:        notes,
		CreatedAt:    now.UTC().Format(time.RFC3339Nano),
		NextDueDate:  nextDueDate.Format(time.RFC3339Nano),
		Area:         string(area),
		RuleType:     cols.ruleType,
		DaysOfWeek:   cols.daysOfWeek,
		DaysOfMonth:  cols.daysOfMonth,
		YearlyDates:  cols.yearlyDates,
		IntervalDays: cols.intervalDays,
	})
	if err != nil {
		return RecurringItem{}, fmt.Errorf("creating recurring item: %w", err)
	}

	return recurringItemFromRow(row)
}

// UpdateTitle sets the title for the recurring item with the given ID and
// returns it. It returns ErrRecurringItemNotFound if no such item exists.
func (i *RecurringItems) UpdateTitle(ctx context.Context, id, title string) (RecurringItem, error) {
	row, err := i.q.UpdateRecurringItemTitle(ctx, sqlcgen.UpdateRecurringItemTitleParams{ID: id, Title: title})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("updating recurring item title: %w", err)
	}

	return recurringItemFromRow(row)
}

// UpdateNotes sets the notes for the recurring item with the given ID and
// returns it. It returns ErrRecurringItemNotFound if no such item exists.
func (i *RecurringItems) UpdateNotes(ctx context.Context, id, notes string) (RecurringItem, error) {
	row, err := i.q.UpdateRecurringItemNotes(ctx, sqlcgen.UpdateRecurringItemNotesParams{ID: id, Notes: notes})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("updating recurring item notes: %w", err)
	}

	return recurringItemFromRow(row)
}

// UpdateArea sets the area for the recurring item with the given ID and
// returns it. It returns ErrRecurringItemNotFound if no such item exists.
func (i *RecurringItems) UpdateArea(ctx context.Context, id string, area Area) (RecurringItem, error) {
	row, err := i.q.UpdateRecurringItemArea(ctx, sqlcgen.UpdateRecurringItemAreaParams{ID: id, Area: string(area)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("updating recurring item area: %w", err)
	}

	return recurringItemFromRow(row)
}

// UpdateRule replaces the recurring item's rule wholesale and recomputes its
// next_due_date from today (recurrence.NextOnCreate), the same as a fresh
// Create - a rule-type change re-anchors the schedule rather than trying to
// reconcile it against the old rule. It returns ErrRecurringItemNotFound if
// no such item exists.
func (i *RecurringItems) UpdateRule(ctx context.Context, id string, rule recurrence.Rule, now time.Time) (RecurringItem, error) {
	nextDueDate, err := recurrence.NextOnCreate(rule, now)
	if err != nil {
		return RecurringItem{}, fmt.Errorf("computing next due date: %w", err)
	}

	cols := ruleColumns(rule)
	row, err := i.q.UpdateRecurringItemRule(ctx, sqlcgen.UpdateRecurringItemRuleParams{
		ID:           id,
		RuleType:     cols.ruleType,
		DaysOfWeek:   cols.daysOfWeek,
		DaysOfMonth:  cols.daysOfMonth,
		YearlyDates:  cols.yearlyDates,
		IntervalDays: cols.intervalDays,
		NextDueDate:  nextDueDate.Format(time.RFC3339Nano),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("updating recurring item rule: %w", err)
	}

	return recurringItemFromRow(row)
}

// Complete advances the recurring item with the given ID to its next
// occurrence (recurrence.NextOnComplete, anchored off the item's current
// due date, with now as the completion moment) and returns the updated
// item. It returns ErrRecurringItemNotFound if no such item exists.
func (i *RecurringItems) Complete(ctx context.Context, id string, now time.Time) (RecurringItem, error) {
	row, err := i.q.GetRecurringItem(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("getting recurring item: %w", err)
	}

	item, err := recurringItemFromRow(row)
	if err != nil {
		return RecurringItem{}, err
	}

	nextDueDate, err := recurrence.NextOnComplete(item.Rule, item.NextDueDate, now)
	if err != nil {
		return RecurringItem{}, fmt.Errorf("computing next due date: %w", err)
	}

	updated, err := i.q.UpdateRecurringItemNextDueDate(ctx, sqlcgen.UpdateRecurringItemNextDueDateParams{
		ID:          id,
		NextDueDate: nextDueDate.Format(time.RFC3339Nano),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecurringItem{}, ErrRecurringItemNotFound
		}
		return RecurringItem{}, fmt.Errorf("completing recurring item: %w", err)
	}

	return recurringItemFromRow(updated)
}

// Delete removes the recurring item with the given ID. It returns
// ErrRecurringItemNotFound if no such item exists.
func (i *RecurringItems) Delete(ctx context.Context, id string) error {
	affected, err := i.q.DeleteRecurringItem(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting recurring item: %w", err)
	}
	if affected == 0 {
		return ErrRecurringItemNotFound
	}

	return nil
}

func recurringItemFromRow(row sqlcgen.RecurringItem) (RecurringItem, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return RecurringItem{}, fmt.Errorf("parsing recurring item created_at: %w", err)
	}

	nextDueDate, err := time.Parse(time.RFC3339Nano, row.NextDueDate)
	if err != nil {
		return RecurringItem{}, fmt.Errorf("parsing recurring item next_due_date: %w", err)
	}

	rule, err := ruleFromRow(row)
	if err != nil {
		return RecurringItem{}, err
	}

	return RecurringItem{
		ID:          row.ID,
		Title:       row.Title,
		Notes:       row.Notes,
		CreatedAt:   createdAt,
		NextDueDate: nextDueDate,
		Area:        Area(row.Area),
		Rule:        rule,
	}, nil
}

// ruleCols is the recurring_items row shape for the rule-specific columns -
// SQLite has no array type, so each repeated recurrence.Rule field is
// flattened to a sorted, comma-separated TEXT column, gated by rule_type.
type ruleCols struct {
	ruleType     string
	daysOfWeek   sql.NullString
	daysOfMonth  sql.NullString
	yearlyDates  sql.NullString
	intervalDays sql.NullInt64
}

// ruleColumns converts a recurrence.Rule (already validated by the caller,
// e.g. via recurrence.NextOnCreate) to its recurring_items column encoding.
func ruleColumns(rule recurrence.Rule) ruleCols {
	switch rule.Type {
	case recurrence.RuleTypeWeekly:
		days := recurrence.NormalizeDaysOfWeek(rule.DaysOfWeek)
		parts := make([]string, len(days))
		for i, d := range days {
			parts[i] = strconv.Itoa(int(d))
		}
		return ruleCols{ruleType: string(rule.Type), daysOfWeek: validString(strings.Join(parts, ","))}
	case recurrence.RuleTypeMonthly:
		days := recurrence.NormalizeDaysOfMonth(rule.DaysOfMonth)
		parts := make([]string, len(days))
		for i, d := range days {
			parts[i] = strconv.Itoa(d)
		}
		return ruleCols{ruleType: string(rule.Type), daysOfMonth: validString(strings.Join(parts, ","))}
	case recurrence.RuleTypeYearly:
		dates := recurrence.NormalizeYearlyDates(rule.YearlyDates)
		parts := make([]string, len(dates))
		for i, d := range dates {
			parts[i] = fmt.Sprintf("%d-%d", d.Month, d.Day)
		}
		return ruleCols{ruleType: string(rule.Type), yearlyDates: validString(strings.Join(parts, ","))}
	case recurrence.RuleTypeInterval:
		return ruleCols{ruleType: string(rule.Type), intervalDays: sql.NullInt64{Int64: int64(rule.IntervalDays), Valid: true}}
	default:
		// Unreachable: rule.Validate() (run by the caller before this is
		// invoked) rejects any other Type.
		return ruleCols{ruleType: string(rule.Type)}
	}
}

// ruleFromRow is ruleColumns' inverse, reconstructing a recurrence.Rule from
// a recurring_items row.
func ruleFromRow(row sqlcgen.RecurringItem) (recurrence.Rule, error) {
	switch recurrence.RuleType(row.RuleType) {
	case recurrence.RuleTypeWeekly:
		days, err := parseIntList(row.DaysOfWeek.String)
		if err != nil {
			return recurrence.Rule{}, fmt.Errorf("parsing recurring item days_of_week: %w", err)
		}
		weekdays := make([]time.Weekday, len(days))
		for i, d := range days {
			weekdays[i] = time.Weekday(d)
		}
		return recurrence.Rule{Type: recurrence.RuleTypeWeekly, DaysOfWeek: weekdays}, nil
	case recurrence.RuleTypeMonthly:
		days, err := parseIntList(row.DaysOfMonth.String)
		if err != nil {
			return recurrence.Rule{}, fmt.Errorf("parsing recurring item days_of_month: %w", err)
		}
		return recurrence.Rule{Type: recurrence.RuleTypeMonthly, DaysOfMonth: days}, nil
	case recurrence.RuleTypeYearly:
		dates, err := parseYearlyDates(row.YearlyDates.String)
		if err != nil {
			return recurrence.Rule{}, fmt.Errorf("parsing recurring item yearly_dates: %w", err)
		}
		return recurrence.Rule{Type: recurrence.RuleTypeYearly, YearlyDates: dates}, nil
	case recurrence.RuleTypeInterval:
		return recurrence.Rule{Type: recurrence.RuleTypeInterval, IntervalDays: int(row.IntervalDays.Int64)}, nil
	default:
		return recurrence.Rule{}, fmt.Errorf("recurring item has unknown rule_type %q", row.RuleType)
	}
}

func validString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

func parseIntList(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	out := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func parseYearlyDates(s string) ([]recurrence.MonthDay, error) {
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	out := make([]recurrence.MonthDay, len(parts))
	for i, p := range parts {
		monthStr, dayStr, ok := strings.Cut(p, "-")
		if !ok {
			return nil, fmt.Errorf("invalid yearly date %q", p)
		}
		month, err := strconv.Atoi(monthStr)
		if err != nil {
			return nil, err
		}
		day, err := strconv.Atoi(dayStr)
		if err != nil {
			return nil, err
		}
		out[i] = recurrence.MonthDay{Month: month, Day: day}
	}
	return out, nil
}
