// Package itemservice implements the shopping.v1.ItemService Connect
// handler over the storage.Items repository.
package itemservice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/shopping/gen/shopping/v1"
	"github.com/liamawhite/shopping/internal/storage"
)

// Service implements shoppingv1connect.ItemServiceHandler.
type Service struct {
	items  *storage.Items
	labels *storage.Labels
}

// New returns a Service backed by items and labels - labels is needed to
// resolve/validate the label an item is created under.
func New(items *storage.Items, labels *storage.Labels) *Service {
	return &Service{items: items, labels: labels}
}

// ListItems returns every item.
func (s *Service) ListItems(ctx context.Context, req *connect.Request[v1.ListItemsRequest]) (*connect.Response[v1.ListItemsResponse], error) {
	items, err := s.items.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListItemsResponse{
		Items: make([]*v1.Item, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toProto(item))
	}

	return connect.NewResponse(resp), nil
}

// CreateItem creates a new item under the given label. A label is required
// - there is no catch-all fallback, so the caller must supply the ID of a
// real, active label.
func (s *Service) CreateItem(ctx context.Context, req *connect.Request[v1.CreateItemRequest]) (*connect.Response[v1.CreateItemResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	labelID := strings.TrimSpace(req.Msg.GetLabelId())
	if labelID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("label_id must not be empty"))
	}

	label, err := s.labels.Get(ctx, labelID)
	if err != nil {
		if errors.Is(err, storage.ErrLabelNotFound) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if label.Archived {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("label is archived"))
	}

	item, err := s.items.Create(ctx, name, label)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateItemResponse{Item: toProto(item)}), nil
}

// DeleteItem removes an item.
func (s *Service) DeleteItem(ctx context.Context, req *connect.Request[v1.DeleteItemRequest]) (*connect.Response[v1.DeleteItemResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.items.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteItemResponse{}), nil
}

// UpdateItemStatus marks an item todo or done. completed_at is set/cleared
// automatically by storage.Items.SetStatus - callers only ever choose the
// status, never a timestamp.
func (s *Service) UpdateItemStatus(ctx context.Context, req *connect.Request[v1.UpdateItemStatusRequest]) (*connect.Response[v1.UpdateItemStatusResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	status, err := fromProtoStatus(req.Msg.GetStatus())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item, err := s.items.SetStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, storage.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// SetStatus's query doesn't join labels (see its doc comment) - fill in
	// the name for the response the same way CreateItem's caller-resolved
	// label does.
	label, err := s.labels.Get(ctx, item.LabelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	item.LabelName = label.Name

	return connect.NewResponse(&v1.UpdateItemStatusResponse{Item: toProto(item)}), nil
}

func toProto(item storage.Item) *v1.Item {
	proto := &v1.Item{
		Id:        item.ID,
		Name:      item.Name,
		LabelId:   item.LabelID,
		LabelName: item.LabelName,
		CreatedAt: timestamppb.New(item.CreatedAt),
		Status:    toProtoStatus(item.Status),
	}
	if item.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*item.CompletedAt)
	}
	return proto
}

func toProtoStatus(status storage.ItemStatus) v1.ItemStatus {
	switch status {
	case storage.ItemStatusDone:
		return v1.ItemStatus_ITEM_STATUS_DONE
	default:
		return v1.ItemStatus_ITEM_STATUS_TODO
	}
}

func fromProtoStatus(status v1.ItemStatus) (storage.ItemStatus, error) {
	switch status {
	case v1.ItemStatus_ITEM_STATUS_TODO:
		return storage.ItemStatusTodo, nil
	case v1.ItemStatus_ITEM_STATUS_DONE:
		return storage.ItemStatusDone, nil
	default:
		return "", fmt.Errorf("status must be TODO or DONE, got %s", status)
	}
}
