package lightscontroller

import (
	"context"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// EventConsumer is a manager.Runnable that drains light events published
// by internal/eventstream.Streamer, patching each Light CR's Status the
// moment the bridge reports on/dimming/color_temperature has changed -
// this is the primary, real-time path for keeping Status in sync; Poller's
// periodic full sweep is now just a drift safety net behind it.
type EventConsumer struct {
	Client client.Client
	Events <-chan lighthue.LightEvent
}

var (
	_ manager.Runnable               = (*EventConsumer)(nil)
	_ manager.LeaderElectionRunnable = (*EventConsumer)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever writes
// Light CRs.
func (c *EventConsumer) NeedLeaderElection() bool { return true }

// Start drains Events until ctx is done.
func (c *EventConsumer) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-c.Events:
			c.handleEvent(ctx, logger, ev)
		}
	}
}

// handleEvent merges ev onto the Light named after ev.LightID's Status. If
// the Light doesn't exist yet (not yet discovered by Poller), skip
// silently - the next Poller tick creates it with current state already
// reflecting whatever this event reported.
func (c *EventConsumer) handleEvent(ctx context.Context, logger logr.Logger, ev lighthue.LightEvent) {
	var light lightsv1alpha1.Light
	if err := c.Client.Get(ctx, client.ObjectKey{Name: ev.LightID}, &light); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to get light for event", "light", ev.LightID)
		}
		return
	}
	light.Status = mergeLightStatus(light.Status, ev, metav1.Now())
	if err := c.Client.Status().Update(ctx, &light); err != nil {
		logger.Error(err, "failed to update light status from event", "light", ev.LightID)
	}
}

// mergeLightStatus computes light's next Status after a (possibly
// partial) event ev - only the fields ev actually reports are overwritten,
// everything else on current is left untouched. Unlike
// switchcontroller's mergedSwitchStatus (which compares event timestamps
// to avoid a slow poll regressing a fresher pushed event), on/brightness/
// color/colorTempK are plain "current value" fields, not
// "which-event-happened-last" fields - both Poller's full GET and a
// pushed event are equally authoritative snapshots of "right now," so a
// plain overwrite-what's-present merge is correct with no regression
// guard needed.
func mergeLightStatus(current lightsv1alpha1.LightStatus, ev lighthue.LightEvent, now metav1.Time) lightsv1alpha1.LightStatus {
	next := current
	if ev.On != nil {
		next.On = *ev.On
	}
	if ev.Brightness != nil {
		next.Brightness = int32(*ev.Brightness)
	}
	if ev.Color != nil {
		next.Color = *ev.Color
	}
	if ev.ColorTempK != nil {
		next.ColorTempK = int32(*ev.ColorTempK)
	}
	// Hearing a live push from the bridge is itself evidence of
	// reachability.
	next.Reachable = true
	next.LastSynced = now
	return next
}
