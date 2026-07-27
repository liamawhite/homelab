// Package groupservice implements the lumenetes.v1.GroupService Connect
// handler by listing Group CRs directly from the Kubernetes API -
// read-only, no local storage of any kind.
package groupservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.GroupServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListGroups returns every Group known to the cluster.
func (s *Service) ListGroups(ctx context.Context, req *connect.Request[v1.ListGroupsRequest]) (*connect.Response[v1.ListGroupsResponse], error) {
	var groups lumenetesv1alpha1.GroupList
	if err := s.client.List(ctx, &groups); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListGroupsResponse{Groups: make([]*v1.Group, 0, len(groups.Items))}
	for _, group := range groups.Items {
		resp.Groups = append(resp.Groups, toProto(group))
	}

	return connect.NewResponse(resp), nil
}

func toProto(group lumenetesv1alpha1.Group) *v1.Group {
	return &v1.Group{
		Id:               group.Name,
		Lights:           group.Spec.Lights,
		ActiveScene:      toProtoActiveScene(group.Spec.ActiveScene),
		MissingLights:    group.Status.MissingLights,
		LightCount:       group.Status.LightCount,
		ActiveSceneError: group.Status.ActiveSceneError,
		LastSynced:       protoutil.Time(group.Status.LastSynced),
	}
}

func toProtoActiveScene(ref *lumenetesv1alpha1.ActiveSceneRef) *v1.ActiveSceneRef {
	if ref == nil {
		return nil
	}
	return &v1.ActiveSceneRef{
		Kind: toProtoKind(ref.Kind),
		Name: ref.Name,
	}
}

func toProtoKind(kind lumenetesv1alpha1.ActiveSceneKind) v1.ActiveSceneKind {
	switch kind {
	case lumenetesv1alpha1.ActiveSceneKindScene:
		return v1.ActiveSceneKind_ACTIVE_SCENE_KIND_SCENE
	case lumenetesv1alpha1.ActiveSceneKindCircadianSchedule:
		return v1.ActiveSceneKind_ACTIVE_SCENE_KIND_CIRCADIAN_SCHEDULE
	case lumenetesv1alpha1.ActiveSceneKindOff:
		return v1.ActiveSceneKind_ACTIVE_SCENE_KIND_OFF
	case lumenetesv1alpha1.ActiveSceneKindReactive:
		return v1.ActiveSceneKind_ACTIVE_SCENE_KIND_REACTIVE
	default:
		return v1.ActiveSceneKind_ACTIVE_SCENE_KIND_UNSPECIFIED
	}
}
