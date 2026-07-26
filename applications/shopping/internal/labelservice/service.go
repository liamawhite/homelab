// Package labelservice implements the shopping.v1.LabelService Connect
// handler over the storage.Labels repository.
package labelservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/shopping/gen/shopping/v1"
	"github.com/liamawhite/shopping/internal/storage"
)

// Service implements shoppingv1connect.LabelServiceHandler.
type Service struct {
	labels *storage.Labels
}

// New returns a Service backed by labels.
func New(labels *storage.Labels) *Service {
	return &Service{labels: labels}
}

// ListLabels returns every label, active and archived.
func (s *Service) ListLabels(ctx context.Context, req *connect.Request[v1.ListLabelsRequest]) (*connect.Response[v1.ListLabelsResponse], error) {
	labels, err := s.labels.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListLabelsResponse{
		Labels: make([]*v1.Label, 0, len(labels)),
	}
	for _, label := range labels {
		resp.Labels = append(resp.Labels, toProto(label))
	}

	return connect.NewResponse(resp), nil
}

// CreateLabel creates a new, active label.
func (s *Service) CreateLabel(ctx context.Context, req *connect.Request[v1.CreateLabelRequest]) (*connect.Response[v1.CreateLabelResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	label, err := s.labels.Create(ctx, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateLabelResponse{Label: toProto(label)}), nil
}

// ArchiveLabel archives a label - it's kept, along with every item that
// references it, just hidden from suggestions/tabs (see storage.Labels.
// Archive's doc comment).
func (s *Service) ArchiveLabel(ctx context.Context, req *connect.Request[v1.ArchiveLabelRequest]) (*connect.Response[v1.ArchiveLabelResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.labels.Archive(ctx, id); err != nil {
		if errors.Is(err, storage.ErrLabelNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.ArchiveLabelResponse{}), nil
}

// RestoreLabel un-archives a label.
func (s *Service) RestoreLabel(ctx context.Context, req *connect.Request[v1.RestoreLabelRequest]) (*connect.Response[v1.RestoreLabelResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.labels.Restore(ctx, id); err != nil {
		if errors.Is(err, storage.ErrLabelNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.RestoreLabelResponse{}), nil
}

// UpdateLabelColor reassigns a label's color to another value from
// storage.LabelPalette.
func (s *Service) UpdateLabelColor(ctx context.Context, req *connect.Request[v1.UpdateLabelColorRequest]) (*connect.Response[v1.UpdateLabelColorResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	label, err := s.labels.SetColor(ctx, id, req.Msg.GetColor())
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrLabelNotFound):
			return nil, connect.NewError(connect.CodeNotFound, err)
		case errors.Is(err, storage.ErrInvalidColor):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&v1.UpdateLabelColorResponse{Label: toProto(label)}), nil
}

func toProto(label storage.Label) *v1.Label {
	return &v1.Label{
		Id:        label.ID,
		Name:      label.Name,
		Archived:  label.Archived,
		CreatedAt: timestamppb.New(label.CreatedAt),
		Color:     label.Color,
	}
}
