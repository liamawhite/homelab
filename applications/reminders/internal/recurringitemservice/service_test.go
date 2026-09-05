package recurringitemservice

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/reminders/gen/reminders/v1"
	"github.com/liamawhite/reminders/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	return newTestServiceWithClock(t, time.Now)
}

func newTestServiceWithClock(t *testing.T, now func() time.Time) *Service {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	return &Service{items: storage.NewRecurringItems(db), now: now}
}

func weeklyRule(days ...v1.Weekday) *v1.RecurrenceRule {
	return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Weekly{Weekly: &v1.WeeklyRule{DaysOfWeek: days}}}
}

func monthlyRule(days ...int32) *v1.RecurrenceRule {
	return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Monthly{Monthly: &v1.MonthlyRule{DaysOfMonth: days}}}
}

func yearlyRule(dates ...*v1.MonthDay) *v1.RecurrenceRule {
	return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Yearly{Yearly: &v1.YearlyRule{Dates: dates}}}
}

func intervalRule(days int32) *v1.RecurrenceRule {
	return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Interval{Interval: &v1.IntervalRule{IntervalDays: days}}}
}

func TestService_CreateRecurringItem_RejectsEmptyTitle(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateRecurringItem(context.Background(), connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "   ",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  weeklyRule(v1.Weekday_WEEKDAY_MONDAY),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateRecurringItem_RejectsUnspecifiedArea(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateRecurringItem(context.Background(), connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Rule:  weeklyRule(v1.Weekday_WEEKDAY_MONDAY),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateRecurringItem_RejectsMissingRule(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateRecurringItem(context.Background(), connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateRecurringItem_RejectsEmptyWeeklyRule(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateRecurringItem(context.Background(), connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  weeklyRule(),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateRecurringItem_Weekly_ComputesNextDueDate(t *testing.T) {
	ctx := context.Background()
	// 2026-08-24 is a Monday.
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  weeklyRule(v1.Weekday_WEEKDAY_MONDAY, v1.Weekday_WEEKDAY_THURSDAY),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	want := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if got := created.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("CreateRecurringItem() next_due_date = %v, want %v", got, want)
	}

	rule := created.Msg.GetItem().GetRule().GetWeekly()
	if rule == nil || len(rule.GetDaysOfWeek()) != 2 {
		t.Fatalf("CreateRecurringItem() rule = %v, want weekly with 2 days", created.Msg.GetItem().GetRule())
	}
}

func TestService_CreateRecurringItem_Monthly_ClampsDayOfMonth(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Pay rent",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  monthlyRule(31),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	// April only has 30 days - day 31 clamps to April 30th.
	want := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	if got := created.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("CreateRecurringItem() next_due_date = %v, want %v", got, want)
	}
}

func TestService_CreateRecurringItem_Yearly_ClampsFeb29(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC) // 2029 is not a leap year
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Leap day thing",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  yearlyRule(&v1.MonthDay{Month: 2, Day: 29}),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	want := time.Date(2029, 2, 28, 0, 0, 0, 0, time.UTC)
	if got := created.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("CreateRecurringItem() next_due_date = %v, want %v", got, want)
	}
}

func TestService_CreateRecurringItem_ThenListRecurringItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	listed, err := svc.ListRecurringItems(ctx, connect.NewRequest(&v1.ListRecurringItemsRequest{}))
	if err != nil {
		t.Fatalf("ListRecurringItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 1 {
		t.Fatalf("ListRecurringItems() = %d items, want 1", len(listed.Msg.GetItems()))
	}
	if listed.Msg.GetItems()[0].GetId() != created.Msg.GetItem().GetId() {
		t.Fatalf("ListRecurringItems()[0].Id = %q, want %q", listed.Msg.GetItems()[0].GetId(), created.Msg.GetItem().GetId())
	}
}

func TestService_CompleteRecurringItem_Weekly_AdvancesFromCurrentDueDate(t *testing.T) {
	ctx := context.Background()
	// 2026-08-24 is a Monday.
	createdAt := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	now := createdAt
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  weeklyRule(v1.Weekday_WEEKDAY_MONDAY),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	// Complete it late, on Thursday the 27th, not Monday itself.
	now = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	completed, err := svc.CompleteRecurringItem(ctx, connect.NewRequest(&v1.CompleteRecurringItemRequest{
		Id: created.Msg.GetItem().GetId(),
	}))
	if err != nil {
		t.Fatalf("CompleteRecurringItem() error = %v", err)
	}

	// The next occurrence is anchored off the due date that was completed
	// (Monday the 24th), not off the late completion date - so it's the
	// following Monday, the 31st, not the 27th+7.
	want := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	if got := completed.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("CompleteRecurringItem() next_due_date = %v, want %v", got, want)
	}
}

func TestService_CompleteRecurringItem_Interval_DriftsFromCompletionDate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}
	if got := created.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(now.Truncate(24*time.Hour)) {
		t.Fatalf("CreateRecurringItem() next_due_date = %v, want %v", got, now)
	}

	// Complete it late, 3 days after the original due date.
	now = time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC)
	completed, err := svc.CompleteRecurringItem(ctx, connect.NewRequest(&v1.CompleteRecurringItemRequest{
		Id: created.Msg.GetItem().GetId(),
	}))
	if err != nil {
		t.Fatalf("CompleteRecurringItem() error = %v", err)
	}

	// The next occurrence drifts 3 days off the completion date (the
	// 27th), landing on the 30th - not 3 days off the original due date
	// (which would also be the 27th, i.e. no drift at all).
	want := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	if got := completed.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("CompleteRecurringItem() next_due_date = %v, want %v", got, want)
	}
}

func TestService_CompleteRecurringItem_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CompleteRecurringItem(context.Background(), connect.NewRequest(&v1.CompleteRecurringItemRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CompleteRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CompleteRecurringItem_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CompleteRecurringItem(context.Background(), connect.NewRequest(&v1.CompleteRecurringItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("CompleteRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateRecurringItemRule_RecomputesNextDueDate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC) // a Monday
	svc := newTestServiceWithClock(t, func() time.Time { return now })

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  weeklyRule(v1.Weekday_WEEKDAY_MONDAY),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	updated, err := svc.UpdateRecurringItemRule(ctx, connect.NewRequest(&v1.UpdateRecurringItemRuleRequest{
		Id:   created.Msg.GetItem().GetId(),
		Rule: intervalRule(5),
	}))
	if err != nil {
		t.Fatalf("UpdateRecurringItemRule() error = %v", err)
	}

	if got := updated.Msg.GetItem().GetRule().GetInterval().GetIntervalDays(); got != 5 {
		t.Fatalf("UpdateRecurringItemRule() interval_days = %d, want 5", got)
	}
	want := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if got := updated.Msg.GetItem().GetNextDueDate(); got == nil || !got.AsTime().Equal(want) {
		t.Fatalf("UpdateRecurringItemRule() next_due_date = %v, want %v", got, want)
	}
}

func TestService_UpdateRecurringItemRule_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateRecurringItemRule(context.Background(), connect.NewRequest(&v1.UpdateRecurringItemRuleRequest{
		Id:   "does-not-exist",
		Rule: intervalRule(3),
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateRecurringItemRule() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateRecurringItemNotes_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateRecurringItemNotes(context.Background(), connect.NewRequest(&v1.UpdateRecurringItemNotesRequest{Id: "  ", Notes: "n"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateRecurringItemNotes() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateRecurringItemNotes_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateRecurringItemNotes(context.Background(), connect.NewRequest(&v1.UpdateRecurringItemNotesRequest{Id: "does-not-exist", Notes: "n"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateRecurringItemNotes() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateRecurringItemNotes_ThenListRecurringItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	updated, err := svc.UpdateRecurringItemNotes(ctx, connect.NewRequest(&v1.UpdateRecurringItemNotesRequest{
		Id:    created.Msg.GetItem().GetId(),
		Notes: "use the blue can",
	}))
	if err != nil {
		t.Fatalf("UpdateRecurringItemNotes() error = %v", err)
	}
	if updated.Msg.GetItem().GetNotes() != "use the blue can" {
		t.Fatalf("UpdateRecurringItemNotes() notes = %q, want %q", updated.Msg.GetItem().GetNotes(), "use the blue can")
	}

	listed, err := svc.ListRecurringItems(ctx, connect.NewRequest(&v1.ListRecurringItemsRequest{}))
	if err != nil {
		t.Fatalf("ListRecurringItems() error = %v", err)
	}
	if listed.Msg.GetItems()[0].GetNotes() != "use the blue can" {
		t.Fatalf("ListRecurringItems()[0].Notes = %q, want %q", listed.Msg.GetItems()[0].GetNotes(), "use the blue can")
	}
}

func TestService_UpdateRecurringItemArea_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateRecurringItemArea(context.Background(), connect.NewRequest(&v1.UpdateRecurringItemAreaRequest{Id: "  ", Area: v1.Area_AREA_WORK}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateRecurringItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateRecurringItemArea_RejectsUnspecifiedArea(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	_, err = svc.UpdateRecurringItemArea(ctx, connect.NewRequest(&v1.UpdateRecurringItemAreaRequest{Id: created.Msg.GetItem().GetId()}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateRecurringItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateRecurringItemArea_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateRecurringItemArea(context.Background(), connect.NewRequest(&v1.UpdateRecurringItemAreaRequest{Id: "does-not-exist", Area: v1.Area_AREA_WORK}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateRecurringItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateRecurringItemArea_ThenListRecurringItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	updated, err := svc.UpdateRecurringItemArea(ctx, connect.NewRequest(&v1.UpdateRecurringItemAreaRequest{
		Id:   created.Msg.GetItem().GetId(),
		Area: v1.Area_AREA_PROFESSIONAL,
	}))
	if err != nil {
		t.Fatalf("UpdateRecurringItemArea() error = %v", err)
	}
	if got := updated.Msg.GetItem().GetArea(); got != v1.Area_AREA_PROFESSIONAL {
		t.Fatalf("UpdateRecurringItemArea() area = %v, want %v", got, v1.Area_AREA_PROFESSIONAL)
	}

	listed, err := svc.ListRecurringItems(ctx, connect.NewRequest(&v1.ListRecurringItemsRequest{}))
	if err != nil {
		t.Fatalf("ListRecurringItems() error = %v", err)
	}
	if got := listed.Msg.GetItems()[0].GetArea(); got != v1.Area_AREA_PROFESSIONAL {
		t.Fatalf("ListRecurringItems()[0].Area = %v, want %v", got, v1.Area_AREA_PROFESSIONAL)
	}
}

func TestService_DeleteRecurringItem_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteRecurringItem(context.Background(), connect.NewRequest(&v1.DeleteRecurringItemRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteRecurringItem_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteRecurringItem(context.Background(), connect.NewRequest(&v1.DeleteRecurringItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteRecurringItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteRecurringItem_ThenListRecurringItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateRecurringItem(ctx, connect.NewRequest(&v1.CreateRecurringItemRequest{
		Title: "Water plants",
		Area:  v1.Area_AREA_PERSONAL,
		Rule:  intervalRule(3),
	}))
	if err != nil {
		t.Fatalf("CreateRecurringItem() error = %v", err)
	}

	if _, err := svc.DeleteRecurringItem(ctx, connect.NewRequest(&v1.DeleteRecurringItemRequest{Id: created.Msg.GetItem().GetId()})); err != nil {
		t.Fatalf("DeleteRecurringItem() error = %v", err)
	}

	listed, err := svc.ListRecurringItems(ctx, connect.NewRequest(&v1.ListRecurringItemsRequest{}))
	if err != nil {
		t.Fatalf("ListRecurringItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 0 {
		t.Fatalf("ListRecurringItems() after delete = %d items, want 0", len(listed.Msg.GetItems()))
	}
}
