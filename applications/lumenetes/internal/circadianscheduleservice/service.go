// Package circadianscheduleservice implements the
// lumenetes.v1.CircadianScheduleService Connect handler by listing
// CircadianSchedule CRs directly from the Kubernetes API - read-only, no
// local storage of any kind.
package circadianscheduleservice

import (
	"context"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	v1 "github.com/liamawhite/lumenetes/gen/lumenetes/v1"
	"github.com/liamawhite/lumenetes/internal/protoutil"
)

// Service implements lumenetesv1connect.CircadianScheduleServiceHandler.
type Service struct {
	client client.Client
}

// New returns a Service backed by c.
func New(c client.Client) *Service {
	return &Service{client: c}
}

// ListCircadianSchedules returns every CircadianSchedule known to the
// cluster.
func (s *Service) ListCircadianSchedules(ctx context.Context, req *connect.Request[v1.ListCircadianSchedulesRequest]) (*connect.Response[v1.ListCircadianSchedulesResponse], error) {
	var schedules lumenetesv1alpha1.CircadianScheduleList
	if err := s.client.List(ctx, &schedules); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListCircadianSchedulesResponse{
		CircadianSchedules: make([]*v1.CircadianSchedule, 0, len(schedules.Items)),
	}
	for _, schedule := range schedules.Items {
		resp.CircadianSchedules = append(resp.CircadianSchedules, toProto(schedule))
	}

	return connect.NewResponse(resp), nil
}

func toProto(schedule lumenetesv1alpha1.CircadianSchedule) *v1.CircadianSchedule {
	keyframes := make([]*v1.CircadianKeyframe, 0, len(schedule.Spec.Keyframes))
	for _, kf := range schedule.Spec.Keyframes {
		keyframes = append(keyframes, &v1.CircadianKeyframe{
			Anchor:        toProtoAnchor(kf.Anchor),
			OffsetMinutes: kf.OffsetMinutes,
			Brightness:    kf.Brightness,
			ColorTempK:    kf.ColorTempK,
			On:            toProtoOnState(kf.On),
		})
	}

	return &v1.CircadianSchedule{
		Id:                schedule.Name,
		Group:             schedule.Spec.Group,
		Latitude:          schedule.Spec.Latitude,
		Longitude:         schedule.Spec.Longitude,
		Keyframes:         keyframes,
		CurrentBrightness: schedule.Status.CurrentBrightness,
		CurrentColorTempK: schedule.Status.CurrentColorTempK,
		ValidationError:   schedule.Status.ValidationError,
		LastSynced:        protoutil.Time(schedule.Status.LastSynced),
	}
}

func toProtoAnchor(anchor lumenetesv1alpha1.CircadianAnchor) v1.CircadianAnchor {
	switch anchor {
	case lumenetesv1alpha1.CircadianAnchorSunrise:
		return v1.CircadianAnchor_CIRCADIAN_ANCHOR_SUNRISE
	case lumenetesv1alpha1.CircadianAnchorSolarNoon:
		return v1.CircadianAnchor_CIRCADIAN_ANCHOR_SOLAR_NOON
	case lumenetesv1alpha1.CircadianAnchorSunset:
		return v1.CircadianAnchor_CIRCADIAN_ANCHOR_SUNSET
	case lumenetesv1alpha1.CircadianAnchorSolarMidnight:
		return v1.CircadianAnchor_CIRCADIAN_ANCHOR_SOLAR_MIDNIGHT
	default:
		return v1.CircadianAnchor_CIRCADIAN_ANCHOR_UNSPECIFIED
	}
}

func toProtoOnState(on lumenetesv1alpha1.CircadianOnState) v1.CircadianOnState {
	switch on {
	case lumenetesv1alpha1.CircadianOnStateOn:
		return v1.CircadianOnState_CIRCADIAN_ON_STATE_ON
	case lumenetesv1alpha1.CircadianOnStateOff:
		return v1.CircadianOnState_CIRCADIAN_ON_STATE_OFF
	default:
		return v1.CircadianOnState_CIRCADIAN_ON_STATE_UNCHANGED
	}
}
