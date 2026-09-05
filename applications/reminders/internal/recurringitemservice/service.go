// Package recurringitemservice implements the reminders.v1.RecurringItemService
// Connect handler over the storage.RecurringItems repository.
package recurringitemservice

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/reminders/gen/reminders/v1"
	"github.com/liamawhite/reminders/internal/recurrence"
	"github.com/liamawhite/reminders/internal/storage"
)

// Service implements remindersv1connect.RecurringItemServiceHandler.
type Service struct {
	items *storage.RecurringItems
	now   func() time.Time
}

// New returns a Service backed by items.
func New(items *storage.RecurringItems) *Service {
	return &Service{items: items, now: time.Now}
}

// ListRecurringItems returns every recurring item.
func (s *Service) ListRecurringItems(ctx context.Context, req *connect.Request[v1.ListRecurringItemsRequest]) (*connect.Response[v1.ListRecurringItemsResponse], error) {
	items, err := s.items.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListRecurringItemsResponse{Items: make([]*v1.RecurringItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toProto(item))
	}

	return connect.NewResponse(resp), nil
}

// CreateRecurringItem creates a new recurring item.
func (s *Service) CreateRecurringItem(ctx context.Context, req *connect.Request[v1.CreateRecurringItemRequest]) (*connect.Response[v1.CreateRecurringItemResponse], error) {
	title := strings.TrimSpace(req.Msg.GetTitle())
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title must not be empty"))
	}

	area, err := fromProtoArea(req.Msg.GetArea())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rule, err := toRecurrenceRule(req.Msg.GetRule())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.Create(ctx, title, strings.TrimSpace(req.Msg.GetNotes()), area, rule, s.now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateRecurringItemResponse{Item: toProto(item)}), nil
}

// UpdateRecurringItemNotes sets the notes for a recurring item.
func (s *Service) UpdateRecurringItemNotes(ctx context.Context, req *connect.Request[v1.UpdateRecurringItemNotesRequest]) (*connect.Response[v1.UpdateRecurringItemNotesResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	item, err := s.items.UpdateNotes(ctx, id, req.Msg.GetNotes())
	if err != nil {
		if errors.Is(err, storage.ErrRecurringItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateRecurringItemNotesResponse{Item: toProto(item)}), nil
}

// UpdateRecurringItemArea sets the area for a recurring item.
func (s *Service) UpdateRecurringItemArea(ctx context.Context, req *connect.Request[v1.UpdateRecurringItemAreaRequest]) (*connect.Response[v1.UpdateRecurringItemAreaResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	area, err := fromProtoArea(req.Msg.GetArea())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.UpdateArea(ctx, id, area)
	if err != nil {
		if errors.Is(err, storage.ErrRecurringItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateRecurringItemAreaResponse{Item: toProto(item)}), nil
}

// UpdateRecurringItemRule replaces the rule for a recurring item, recomputing
// its next due date from today.
func (s *Service) UpdateRecurringItemRule(ctx context.Context, req *connect.Request[v1.UpdateRecurringItemRuleRequest]) (*connect.Response[v1.UpdateRecurringItemRuleResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	rule, err := toRecurrenceRule(req.Msg.GetRule())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.UpdateRule(ctx, id, rule, s.now())
	if err != nil {
		if errors.Is(err, storage.ErrRecurringItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateRecurringItemRuleResponse{Item: toProto(item)}), nil
}

// CompleteRecurringItem advances a recurring item to its next occurrence.
func (s *Service) CompleteRecurringItem(ctx context.Context, req *connect.Request[v1.CompleteRecurringItemRequest]) (*connect.Response[v1.CompleteRecurringItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	item, err := s.items.Complete(ctx, id, s.now())
	if err != nil {
		if errors.Is(err, storage.ErrRecurringItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CompleteRecurringItemResponse{Item: toProto(item)}), nil
}

// DeleteRecurringItem deletes a recurring item.
func (s *Service) DeleteRecurringItem(ctx context.Context, req *connect.Request[v1.DeleteRecurringItemRequest]) (*connect.Response[v1.DeleteRecurringItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.items.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrRecurringItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteRecurringItemResponse{}), nil
}

func toProto(item storage.RecurringItem) *v1.RecurringItem {
	return &v1.RecurringItem{
		Id:          item.ID,
		Title:       item.Title,
		CreatedAt:   timestamppb.New(item.CreatedAt),
		Notes:       item.Notes,
		NextDueDate: timestamppb.New(item.NextDueDate),
		Area:        toProtoArea(item.Area),
		Rule:        toProtoRule(item.Rule),
	}
}

func toProtoArea(area storage.Area) v1.Area {
	switch area {
	case storage.AreaPersonal:
		return v1.Area_AREA_PERSONAL
	case storage.AreaFamily:
		return v1.Area_AREA_FAMILY
	case storage.AreaWork:
		return v1.Area_AREA_WORK
	case storage.AreaProfessional:
		return v1.Area_AREA_PROFESSIONAL
	default:
		return v1.Area_AREA_UNSPECIFIED
	}
}

func fromProtoArea(area v1.Area) (storage.Area, error) {
	switch area {
	case v1.Area_AREA_PERSONAL:
		return storage.AreaPersonal, nil
	case v1.Area_AREA_FAMILY:
		return storage.AreaFamily, nil
	case v1.Area_AREA_WORK:
		return storage.AreaWork, nil
	case v1.Area_AREA_PROFESSIONAL:
		return storage.AreaProfessional, nil
	default:
		return "", errors.New("area must be one of personal, family, work, professional")
	}
}

// toRecurrenceRule converts a wire RecurrenceRule (exactly one oneof case
// set) to a recurrence.Rule, validating it in the process.
func toRecurrenceRule(rule *v1.RecurrenceRule) (recurrence.Rule, error) {
	var r recurrence.Rule

	switch variant := rule.GetRule().(type) {
	case *v1.RecurrenceRule_Weekly:
		days := make([]time.Weekday, 0, len(variant.Weekly.GetDaysOfWeek()))
		for _, d := range variant.Weekly.GetDaysOfWeek() {
			weekday, err := fromProtoWeekday(d)
			if err != nil {
				return recurrence.Rule{}, err
			}
			days = append(days, weekday)
		}
		r = recurrence.Rule{Type: recurrence.RuleTypeWeekly, DaysOfWeek: days}
	case *v1.RecurrenceRule_Monthly:
		days := make([]int, 0, len(variant.Monthly.GetDaysOfMonth()))
		for _, d := range variant.Monthly.GetDaysOfMonth() {
			days = append(days, int(d))
		}
		r = recurrence.Rule{Type: recurrence.RuleTypeMonthly, DaysOfMonth: days}
	case *v1.RecurrenceRule_Yearly:
		dates := make([]recurrence.MonthDay, 0, len(variant.Yearly.GetDates()))
		for _, d := range variant.Yearly.GetDates() {
			dates = append(dates, recurrence.MonthDay{Month: int(d.GetMonth()), Day: int(d.GetDay())})
		}
		r = recurrence.Rule{Type: recurrence.RuleTypeYearly, YearlyDates: dates}
	case *v1.RecurrenceRule_Interval:
		r = recurrence.Rule{Type: recurrence.RuleTypeInterval, IntervalDays: int(variant.Interval.GetIntervalDays())}
	default:
		return recurrence.Rule{}, errors.New("rule must set exactly one of weekly, monthly, yearly, or interval")
	}

	if err := r.Validate(); err != nil {
		return recurrence.Rule{}, err
	}
	return r, nil
}

func toProtoRule(rule recurrence.Rule) *v1.RecurrenceRule {
	switch rule.Type {
	case recurrence.RuleTypeWeekly:
		days := make([]v1.Weekday, len(rule.DaysOfWeek))
		for i, d := range rule.DaysOfWeek {
			days[i] = toProtoWeekday(d)
		}
		return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Weekly{Weekly: &v1.WeeklyRule{DaysOfWeek: days}}}
	case recurrence.RuleTypeMonthly:
		days := make([]int32, len(rule.DaysOfMonth))
		for i, d := range rule.DaysOfMonth {
			days[i] = int32(d)
		}
		return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Monthly{Monthly: &v1.MonthlyRule{DaysOfMonth: days}}}
	case recurrence.RuleTypeYearly:
		dates := make([]*v1.MonthDay, len(rule.YearlyDates))
		for i, d := range rule.YearlyDates {
			dates[i] = &v1.MonthDay{Month: int32(d.Month), Day: int32(d.Day)}
		}
		return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Yearly{Yearly: &v1.YearlyRule{Dates: dates}}}
	case recurrence.RuleTypeInterval:
		return &v1.RecurrenceRule{Rule: &v1.RecurrenceRule_Interval{Interval: &v1.IntervalRule{IntervalDays: int32(rule.IntervalDays)}}}
	default:
		return nil
	}
}

// fromProtoWeekday maps the wire Weekday enum (1=Sunday..7=Saturday) to
// time.Weekday (0=Sunday..6=Saturday).
func fromProtoWeekday(w v1.Weekday) (time.Weekday, error) {
	if w == v1.Weekday_WEEKDAY_UNSPECIFIED || w < v1.Weekday_WEEKDAY_SUNDAY || w > v1.Weekday_WEEKDAY_SATURDAY {
		return 0, errors.New("weekday must not be unspecified")
	}
	return time.Weekday(w - 1), nil
}

func toProtoWeekday(w time.Weekday) v1.Weekday {
	return v1.Weekday(w + 1)
}
