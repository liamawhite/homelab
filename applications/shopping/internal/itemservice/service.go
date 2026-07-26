// Package itemservice implements the shopping.v1.ItemService Connect
// handler over the storage.Items repository.
package itemservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/shopping/gen/shopping/v1"
	"github.com/liamawhite/shopping/internal/storage"
)

// Service implements shoppingv1connect.ItemServiceHandler.
type Service struct {
	items *storage.Items
}

// New returns a Service backed by items.
func New(items *storage.Items) *Service {
	return &Service{items: items}
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

// CreateItem creates a new item.
func (s *Service) CreateItem(ctx context.Context, req *connect.Request[v1.CreateItemRequest]) (*connect.Response[v1.CreateItemResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	item, err := s.items.Create(ctx, name)
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

func toProto(item storage.Item) *v1.Item {
	return &v1.Item{
		Id:        item.ID,
		Name:      item.Name,
		CreatedAt: timestamppb.New(item.CreatedAt),
	}
}
