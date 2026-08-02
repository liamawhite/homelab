// Package oneoffitemservice implements the reminders.v1.OneOffItemService
// Connect handler over the storage.OneOffItems repository.
package oneoffitemservice

import (
	"context"
	"errors"
	"strings"

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

	item, err := s.items.Create(ctx, title)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateOneOffItemResponse{Item: toProto(item)}), nil
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
	return &v1.OneOffItem{
		Id:        item.ID,
		Title:     item.Title,
		CreatedAt: timestamppb.New(item.CreatedAt),
	}
}
