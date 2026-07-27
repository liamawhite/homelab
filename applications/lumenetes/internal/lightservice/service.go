// Package lightservice implements the lumenetes.v1.LightService Connect
// handler by listing Light CRs directly from the Kubernetes API -
// read-only, no local storage of any kind.
package lightservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.LightServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListLights returns every Light known to the cluster.
func (s *Service) ListLights(ctx context.Context, req *connect.Request[v1.ListLightsRequest]) (*connect.Response[v1.ListLightsResponse], error) {
	var lights lumenetesv1alpha1.LightList
	if err := s.client.List(ctx, &lights); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListLightsResponse{Lights: make([]*v1.Light, 0, len(lights.Items))}
	for _, light := range lights.Items {
		resp.Lights = append(resp.Lights, toProto(light))
	}

	return connect.NewResponse(resp), nil
}

func toProto(light lumenetesv1alpha1.Light) *v1.Light {
	return &v1.Light{
		Id:                 light.Name,
		Name:               light.Status.Name,
		BridgeId:           light.Status.BridgeID,
		ObservedOn:         light.Status.On,
		ObservedBrightness: light.Status.Brightness,
		ObservedColor:      light.Status.Color,
		ObservedColorTempK: light.Status.ColorTempK,
		DesiredOn:          light.Spec.On,
		DesiredBrightness:  light.Spec.Brightness,
		DesiredColor:       light.Spec.Color,
		DesiredColorTempK:  light.Spec.ColorTempK,
		Reactive:           light.Spec.Reactive,
		FixtureType:        light.Status.FixtureType,
		Product:            light.Status.Product,
		Model:              light.Status.Model,
		Reachable:          light.Status.Reachable,
		LastSynced:         protoutil.Time(light.Status.LastSynced),
		LastEnactAttempt:   protoutil.Time(light.Status.LastEnactAttempt),
		EnactError:         light.Status.EnactError,
	}
}
