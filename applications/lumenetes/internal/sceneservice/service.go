// Package sceneservice implements the lumenetes.v1.SceneService Connect
// handler by listing Scene CRs directly from the Kubernetes API - read-only,
// no local storage of any kind.
package sceneservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.SceneServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListScenes returns every Scene known to the cluster.
func (s *Service) ListScenes(ctx context.Context, req *connect.Request[v1.ListScenesRequest]) (*connect.Response[v1.ListScenesResponse], error) {
	var scenes lumenetesv1alpha1.SceneList
	if err := s.client.List(ctx, &scenes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListScenesResponse{Scenes: make([]*v1.Scene, 0, len(scenes.Items))}
	for _, scene := range scenes.Items {
		resp.Scenes = append(resp.Scenes, toProto(scene))
	}

	return connect.NewResponse(resp), nil
}

func toProto(scene lumenetesv1alpha1.Scene) *v1.Scene {
	lights := make([]*v1.SceneLightState, 0, len(scene.Spec.Lights))
	for _, state := range scene.Spec.Lights {
		lights = append(lights, &v1.SceneLightState{
			Name:       state.Name,
			On:         state.On,
			Brightness: state.Brightness,
			Color:      state.Color,
			ColorTempK: state.ColorTempK,
		})
	}

	return &v1.Scene{
		Id:            scene.Name,
		Group:         scene.Spec.Group,
		Lights:        lights,
		InvalidLights: scene.Status.InvalidLights,
		LastSynced:    protoutil.Time(scene.Status.LastSynced),
	}
}
