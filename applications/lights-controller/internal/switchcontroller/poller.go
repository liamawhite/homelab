// Package switchcontroller implements the poll-and-sync loop that keeps
// Switch custom resources' inventory (discovery/battery/reachability) in
// sync with live Hue bridge state, the real-time eventstream Streamer that
// keeps their LastEvent/LastEventTime current, and the Reconciler that
// turns a new button event into Light spec changes per each Switch's
// declared bindings.
package switchcontroller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// bridgeIDLabel labels every Switch CR with the bridge it came from, so a
// bridge's switches can be listed/scoped for GC and reachability updates
// without listing every Switch in the cluster. Deliberately a separate
// constant from lightscontroller's (rather than shared/exported) - kept as
// an independent, parallel implementation, matching this repo's existing
// precedent of not generalizing structurally-similar-but-independent
// pollers (hubcontroller.Poller vs lightscontroller.Poller).
const bridgeIDLabel = "lights.homelab.internal/bridge-id"

// Poller is a manager.Runnable that periodically syncs every paired
// bridge's switches into Switch custom resources: creating/updating one
// per button, marking a bridge's switches unreachable (never deleting
// them) if that bridge fails to respond, and deleting Switch CRs for
// buttons no longer present on a bridge that *did* respond this cycle.
// Discovery/battery/reachability freshness doesn't need to be tight -
// real-time LastEvent/LastEventTime updates are Streamer's job, not this
// Poller's.
type Poller struct {
	Client       client.Client
	Bridges      []bridges.Config
	PollInterval time.Duration
}

var (
	_ manager.Runnable               = (*Poller)(nil)
	_ manager.LeaderElectionRunnable = (*Poller)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever talks to
// bridges or writes Switch CRs - relevant now defensively, essential if
// replicas are ever raised above one.
func (p *Poller) NeedLeaderElection() bool { return true }

// Start runs one sync immediately, then on every PollInterval tick until
// ctx is done.
func (p *Poller) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	p.sync(ctx, logger)

	ticker := time.NewTicker(p.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			p.sync(ctx, logger)
		}
	}
}

func (p *Poller) sync(ctx context.Context, logger logr.Logger) {
	for _, b := range p.Bridges {
		p.syncBridge(ctx, logger, b)
	}
}

// syncBridge reads b's current IP from its HueBridge CR (maintained by
// hub-controller), fetches its switches, and reconciles Switch CRs against
// that result. A missing/unreachable HueBridge, or a failure to actually
// reach the bridge at its recorded IP, marks this bridge's existing
// switches unreachable and returns without touching GC - a bridge blip
// must never look like "these switches were removed."
func (p *Poller) syncBridge(ctx context.Context, logger logr.Logger, b bridges.Config) {
	var hueBridge lightsv1alpha1.HueBridge
	if err := p.Client.Get(ctx, client.ObjectKey{Name: bridges.ResourceName(b.ID)}, &hueBridge); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to get HueBridge", "bridge", b.ID)
		}
		p.markUnreachable(ctx, logger, b.ID)
		return
	}
	if !hueBridge.Status.Reachable || hueBridge.Status.IP == "" {
		logger.Info("bridge not reachable this cycle", "bridge", b.ID)
		p.markUnreachable(ctx, logger, b.ID)
		return
	}

	switches, err := lighthue.FetchSwitches(ctx, hueBridge.Status.IP, b.ID, b.AppKey)
	if err != nil {
		logger.Error(err, "failed to fetch switches from bridge", "bridge", b.ID, "ip", hueBridge.Status.IP)
		p.markUnreachable(ctx, logger, b.ID)
		return
	}

	seen := make(map[string]bool, len(switches))
	for _, s := range switches {
		seen[s.ID] = true
		p.upsert(ctx, logger, s)
	}
	p.gc(ctx, logger, b.ID, seen)
}

// upsert ensures a Switch CR named after s.ID exists and carries s's
// current status. Unlike lightscontroller.Poller.upsert, Spec is never
// seeded here - a Switch's bindings are pure user configuration from day
// one (see SwitchSpec's doc comment).
func (p *Poller) upsert(ctx context.Context, logger logr.Logger, s lighthue.Switch) {
	sw := &lightsv1alpha1.Switch{ObjectMeta: metav1.ObjectMeta{Name: s.ID}}

	_, err := controllerutil.CreateOrUpdate(ctx, p.Client, sw, func() error {
		if sw.Labels == nil {
			sw.Labels = map[string]string{}
		}
		sw.Labels[bridgeIDLabel] = s.BridgeID
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to upsert switch", "switch", s.ID)
		return
	}

	sw.Status = mergedSwitchStatus(sw.Status, s, metav1.Now())
	if err := p.Client.Status().Update(ctx, sw); err != nil {
		logger.Error(err, "failed to update switch status", "switch", s.ID)
	}
}

// mergedSwitchStatus computes sw's next Status after a poll of s, keeping
// whatever LastEvent/LastEventTime/LastHandledEventTime is already on
// current (which may be fresher, written by Streamer) rather than
// regressing it - the bridge poll and the SSE push both ultimately derive
// from the same underlying button_report.updated timestamp, so this is a
// "never go backwards" merge, not "trust whichever source last wrote."
func mergedSwitchStatus(current lightsv1alpha1.SwitchStatus, s lighthue.Switch, now metav1.Time) lightsv1alpha1.SwitchStatus {
	next := lightsv1alpha1.SwitchStatus{
		Name:                 s.Name,
		BridgeID:             s.BridgeID,
		ControlID:            int32(s.ControlID),
		LastEvent:            current.LastEvent,
		LastEventTime:        current.LastEventTime,
		LastHandledEventTime: current.LastHandledEventTime,
		Battery:              int32(s.Battery),
		Product:              s.Product,
		Model:                s.Model,
		Reachable:            true,
		LastSynced:           now,
	}
	if s.LastEventTime.After(current.LastEventTime.Time) {
		next.LastEvent = s.LastEvent
		next.LastEventTime = metav1.NewTime(s.LastEventTime)
	}
	return next
}

// markUnreachable flips status.reachable to false for every Switch CR
// labeled with bridgeID, leaving the rest of their status (last-known
// event/battery/etc.) untouched.
func (p *Poller) markUnreachable(ctx context.Context, logger logr.Logger, bridgeID string) {
	var list lightsv1alpha1.SwitchList
	if err := p.Client.List(ctx, &list, client.MatchingLabels{bridgeIDLabel: bridgeID}); err != nil {
		logger.Error(err, "failed to list switches to mark unreachable", "bridge", bridgeID)
		return
	}
	for i := range list.Items {
		sw := &list.Items[i]
		if !sw.Status.Reachable {
			continue
		}
		sw.Status.Reachable = false
		if err := p.Client.Status().Update(ctx, sw); err != nil {
			logger.Error(err, "failed to mark switch unreachable", "switch", sw.Name)
		}
	}
}

// gc deletes Switch CRs labeled with bridgeID whose name isn't in seen -
// only called for a bridge that was successfully polled this cycle, so
// "not in seen" reliably means "no longer on the bridge," not "bridge was
// unreachable."
func (p *Poller) gc(ctx context.Context, logger logr.Logger, bridgeID string, seen map[string]bool) {
	var list lightsv1alpha1.SwitchList
	if err := p.Client.List(ctx, &list, client.MatchingLabels{bridgeIDLabel: bridgeID}); err != nil {
		logger.Error(err, "failed to list switches for gc", "bridge", bridgeID)
		return
	}
	for i := range list.Items {
		sw := &list.Items[i]
		if seen[sw.Name] {
			continue
		}
		if err := p.Client.Delete(ctx, sw); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete stale switch", "switch", sw.Name)
		}
	}
}
