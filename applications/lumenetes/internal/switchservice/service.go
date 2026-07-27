// Package switchservice implements the lumenetes.v1.SwitchService Connect
// handler by listing Switch CRs directly from the Kubernetes API -
// read-only, no local storage of any kind.
package switchservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.SwitchServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListSwitches returns every Switch known to the cluster.
func (s *Service) ListSwitches(ctx context.Context, req *connect.Request[v1.ListSwitchesRequest]) (*connect.Response[v1.ListSwitchesResponse], error) {
	var switches lumenetesv1alpha1.SwitchList
	if err := s.client.List(ctx, &switches); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListSwitchesResponse{Switches: make([]*v1.Switch, 0, len(switches.Items))}
	for _, sw := range switches.Items {
		resp.Switches = append(resp.Switches, toProto(sw))
	}

	return connect.NewResponse(resp), nil
}

func toProto(sw lumenetesv1alpha1.Switch) *v1.Switch {
	bindings := make([]*v1.SwitchBinding, 0, len(sw.Spec.Bindings))
	for _, binding := range sw.Spec.Bindings {
		bindings = append(bindings, toProtoBinding(binding))
	}

	return &v1.Switch{
		Id:            sw.Name,
		Name:          sw.Status.Name,
		BridgeId:      sw.Status.BridgeID,
		ControlId:     sw.Status.ControlID,
		LastEvent:     sw.Status.LastEvent,
		LastEventTime: protoutil.Time(sw.Status.LastEventTime),
		Battery:       sw.Status.Battery,
		Product:       sw.Status.Product,
		Model:         sw.Status.Model,
		Reachable:     sw.Status.Reachable,
		LastSynced:    protoutil.Time(sw.Status.LastSynced),
		Bindings:      bindings,
	}
}

func toProtoBinding(binding lumenetesv1alpha1.SwitchBinding) *v1.SwitchBinding {
	action := binding.Action
	return &v1.SwitchBinding{
		Event: binding.Event,
		Action: &v1.SwitchAction{
			TargetLights:    action.TargetLights,
			On:              action.On,
			Toggle:          action.Toggle,
			Brightness:      action.Brightness,
			BrightnessDelta: action.BrightnessDelta,
			Color:           action.Color,
			ColorTempK:      action.ColorTempK,
		},
	}
}
