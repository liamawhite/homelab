// Package userservice implements the finances.v1.UserService Connect
// handler over the storage.Users repository.
package userservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/finances/gen/finances/v1"
	"github.com/liamawhite/finances/internal/storage"
)

// Service implements financesv1connect.UserServiceHandler.
type Service struct {
	users *storage.Users
}

// New returns a Service backed by users.
func New(users *storage.Users) *Service {
	return &Service{users: users}
}

// ListUsers returns every user.
func (s *Service) ListUsers(ctx context.Context, req *connect.Request[v1.ListUsersRequest]) (*connect.Response[v1.ListUsersResponse], error) {
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListUsersResponse{
		Users: make([]*v1.User, 0, len(users)),
	}
	for _, user := range users {
		resp.Users = append(resp.Users, toProto(user))
	}

	return connect.NewResponse(resp), nil
}

// CreateUser creates a new user.
func (s *Service) CreateUser(ctx context.Context, req *connect.Request[v1.CreateUserRequest]) (*connect.Response[v1.CreateUserResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	user, err := s.users.Create(ctx, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateUserResponse{User: toProto(user)}), nil
}

// DeleteUser removes a user.
func (s *Service) DeleteUser(ctx context.Context, req *connect.Request[v1.DeleteUserRequest]) (*connect.Response[v1.DeleteUserResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.users.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteUserResponse{}), nil
}

func toProto(user storage.User) *v1.User {
	return &v1.User{
		Id:        user.ID,
		Name:      user.Name,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}
}
