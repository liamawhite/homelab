// Package oneoffitemservice implements the reminders.v1.OneOffItemService
// Connect handler over the storage.OneOffItems repository.
package oneoffitemservice

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/reminders/gen/reminders/v1"
	"github.com/liamawhite/reminders/internal/storage"
)

// Service implements remindersv1connect.OneOffItemServiceHandler.
type Service struct {
	items *storage.OneOffItems
}

// New returns a Service backed by items.
func New(items *storage.OneOffItems) *Service {
	return &Service{items: items}
}

// ListOneOffItems returns every one-off item.
func (s *Service) ListOneOffItems(ctx context.Context, req *connect.Request[v1.ListOneOffItemsRequest]) (*connect.Response[v1.ListOneOffItemsResponse], error) {
	items, err := s.items.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListOneOffItemsResponse{Items: make([]*v1.OneOffItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toProto(item))
	}

	return connect.NewResponse(resp), nil
}

// CreateOneOffItem creates a new one-off item.
func (s *Service) CreateOneOffItem(ctx context.Context, req *connect.Request[v1.CreateOneOffItemRequest]) (*connect.Response[v1.CreateOneOffItemResponse], error) {
	title := strings.TrimSpace(req.Msg.GetTitle())
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title must not be empty"))
	}

	var dueDate *time.Time
	if req.Msg.GetDueDate() != nil {
		t := req.Msg.GetDueDate().AsTime()
		dueDate = &t
	}

	area, err := fromProtoArea(req.Msg.GetArea())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.Create(ctx, title, strings.TrimSpace(req.Msg.GetNotes()), dueDate, area)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateOneOffItemResponse{Item: toProto(item)}), nil
}

// UpdateOneOffItemNotes sets the notes for a one-off item.
func (s *Service) UpdateOneOffItemNotes(ctx context.Context, req *connect.Request[v1.UpdateOneOffItemNotesRequest]) (*connect.Response[v1.UpdateOneOffItemNotesResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	item, err := s.items.UpdateNotes(ctx, id, req.Msg.GetNotes())
	if err != nil {
		if errors.Is(err, storage.ErrOneOffItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateOneOffItemNotesResponse{Item: toProto(item)}), nil
}

// UpdateOneOffItemDueDate sets or clears the due date for a one-off item.
func (s *Service) UpdateOneOffItemDueDate(ctx context.Context, req *connect.Request[v1.UpdateOneOffItemDueDateRequest]) (*connect.Response[v1.UpdateOneOffItemDueDateResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	var dueDate *time.Time
	if req.Msg.GetDueDate() != nil {
		t := req.Msg.GetDueDate().AsTime()
		dueDate = &t
	}

	item, err := s.items.UpdateDueDate(ctx, id, dueDate)
	if err != nil {
		if errors.Is(err, storage.ErrOneOffItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateOneOffItemDueDateResponse{Item: toProto(item)}), nil
}

// UpdateOneOffItemStatus sets the status for a one-off item.
func (s *Service) UpdateOneOffItemStatus(ctx context.Context, req *connect.Request[v1.UpdateOneOffItemStatusRequest]) (*connect.Response[v1.UpdateOneOffItemStatusResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	status, err := fromProtoStatus(req.Msg.GetStatus())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.UpdateStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, storage.ErrOneOffItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateOneOffItemStatusResponse{Item: toProto(item)}), nil
}

// UpdateOneOffItemArea sets the area for a one-off item.
func (s *Service) UpdateOneOffItemArea(ctx context.Context, req *connect.Request[v1.UpdateOneOffItemAreaRequest]) (*connect.Response[v1.UpdateOneOffItemAreaResponse], error) {
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
		if errors.Is(err, storage.ErrOneOffItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateOneOffItemAreaResponse{Item: toProto(item)}), nil
}

// DeleteOneOffItem deletes a one-off item.
func (s *Service) DeleteOneOffItem(ctx context.Context, req *connect.Request[v1.DeleteOneOffItemRequest]) (*connect.Response[v1.DeleteOneOffItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.items.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrOneOffItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteOneOffItemResponse{}), nil
}

func toProto(item storage.OneOffItem) *v1.OneOffItem {
	p := &v1.OneOffItem{
		Id:        item.ID,
		Title:     item.Title,
		CreatedAt: timestamppb.New(item.CreatedAt),
		Notes:     item.Notes,
		Area:      toProtoArea(item.Area),
		Status:    toProtoStatus(item.Status),
	}
	if item.DueDate != nil {
		p.DueDate = timestamppb.New(*item.DueDate)
	}
	return p
}

func toProtoStatus(status storage.OneOffItemStatus) v1.OneOffItemStatus {
	switch status {
	case storage.OneOffItemStatusOpen:
		return v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_OPEN
	case storage.OneOffItemStatusCompleted:
		return v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED
	default:
		return v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_UNSPECIFIED
	}
}

func fromProtoStatus(status v1.OneOffItemStatus) (storage.OneOffItemStatus, error) {
	switch status {
	case v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_OPEN:
		return storage.OneOffItemStatusOpen, nil
	case v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED:
		return storage.OneOffItemStatusCompleted, nil
	default:
		return "", errors.New("status must be one of open, completed")
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
