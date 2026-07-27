// Package bridgeservice implements the lumenetes.v1.BridgeService Connect
// handler by listing HueBridge CRs directly from the Kubernetes API -
// read-only, no local storage of any kind.
package bridgeservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.BridgeServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListBridges returns every HueBridge known to the cluster.
func (s *Service) ListBridges(ctx context.Context, req *connect.Request[v1.ListBridgesRequest]) (*connect.Response[v1.ListBridgesResponse], error) {
	var bridges lumenetesv1alpha1.HueBridgeList
	if err := s.client.List(ctx, &bridges); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListBridgesResponse{Bridges: make([]*v1.Bridge, 0, len(bridges.Items))}
	for _, bridge := range bridges.Items {
		resp.Bridges = append(resp.Bridges, toProto(bridge))
	}

	return connect.NewResponse(resp), nil
}

func toProto(bridge lumenetesv1alpha1.HueBridge) *v1.Bridge {
	return &v1.Bridge{
		Id:           bridge.Name,
		Name:         bridge.Status.Name,
		Ip:           bridge.Status.IP,
		ModelId:      bridge.Status.ModelID,
		ApiVersion:   bridge.Status.APIVersion,
		SwVersion:    bridge.Status.SWVersion,
		Mac:          bridge.Status.MAC,
		Reachable:    bridge.Status.Reachable,
		LastResolved: protoutil.Time(bridge.Status.LastResolved),
	}
}
