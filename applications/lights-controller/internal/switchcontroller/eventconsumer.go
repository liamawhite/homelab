package switchcontroller

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

// EventConsumer is a manager.Runnable that drains button events published
// by internal/eventstream.Streamer, patching each Switch CR's
// Status.LastEvent/LastEventTime the moment a button press is observed -
// this status write is what triggers Reconciler via the normal
// controller-runtime watch, so no custom trigger channel is needed between
// EventConsumer and Reconciler (unlike lightscontroller's old Poller/
// Reconciler TriggerSync, which existed for the opposite direction and has
// since been removed in favor of the same eventstream-based approach).
type EventConsumer struct {
	Client client.Client
	Events <-chan lighthue.ButtonEvent
}

var (
	_ manager.Runnable               = (*EventConsumer)(nil)
	_ manager.LeaderElectionRunnable = (*EventConsumer)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever writes
// Switch CRs.
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

// handleEvent patches the Switch named after ev.ButtonID with its new
// LastEvent/LastEventTime. If the Switch doesn't exist yet (not yet
// discovered by Poller), skip silently - the next Poller tick creates it
// with this same event already current via FetchSwitches, so nothing is
// lost, just delayed.
func (c *EventConsumer) handleEvent(ctx context.Context, logger logr.Logger, ev lighthue.ButtonEvent) {
	var sw lightsv1alpha1.Switch
	if err := c.Client.Get(ctx, client.ObjectKey{Name: ev.ButtonID}, &sw); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to get switch for event", "switch", ev.ButtonID)
		}
		return
	}
	sw.Status.LastEvent = ev.Event
	sw.Status.LastEventTime = metav1.NewTime(ev.Time)
	if err := c.Client.Status().Update(ctx, &sw); err != nil {
		logger.Error(err, "failed to update switch status from event", "switch", ev.ButtonID, "event", ev.Event)
		return
	}
	logger.Info("switch event received", "switch", ev.ButtonID, "event", ev.Event, "time", ev.Time)
}
