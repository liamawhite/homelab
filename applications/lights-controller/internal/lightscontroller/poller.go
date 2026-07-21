// Package lightscontroller implements the poll-and-sync loop that keeps
// Light custom resources in sync with live Hue bridge state.
package lightscontroller

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

// bridgeIDLabel labels every Light CR with the bridge it came from, so a
// bridge's lights can be listed/scoped for GC and reachability updates
// without listing every Light in the cluster.
const bridgeIDLabel = "lights.homelab.internal/bridge-id"

// LightRef identifies a single light on a single bridge - used by Trigger
// to ask Poller for an out-of-band refresh of just that light, without
// waiting for the next regular tick.
type LightRef struct {
	BridgeID string
	LightID  string
}

// Poller is a manager.Runnable that periodically syncs every paired
// bridge's lights into Light custom resources: creating/updating one per
// light, marking a bridge's lights unreachable (never deleting them) if
// that bridge fails to respond, and deleting Light CRs for lights no
// longer present on a bridge that *did* respond this cycle. Discovery
// isn't done here - see internal/hubcontroller; each bridge's current IP
// is read from the HueBridge CR hub-controller maintains.
type Poller struct {
	Client       client.Client
	Bridges      []bridges.Config
	PollInterval time.Duration
	// Trigger, if non-nil, lets Reconciler ask for an out-of-band refresh
	// of one light sooner than the next regular tick - e.g. right after
	// successfully enacting a spec change, so Status converges within
	// seconds instead of waiting up to PollInterval.
	Trigger <-chan LightRef
}

var (
	_ manager.Runnable               = (*Poller)(nil)
	_ manager.LeaderElectionRunnable = (*Poller)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever talks to
// bridges or writes Light CRs - relevant now defensively, essential if
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
		case ref := <-p.Trigger:
			p.syncOne(ctx, logger, ref.BridgeID, ref.LightID)
		}
	}
}

func (p *Poller) sync(ctx context.Context, logger logr.Logger) {
	for _, b := range p.Bridges {
		p.syncBridge(ctx, logger, b)
	}
}

// syncBridge reads b's current IP from its HueBridge CR (maintained by
// hub-controller), fetches its lights, and reconciles Light CRs against
// that result. A missing/unreachable HueBridge, or a failure to actually
// reach the bridge at its recorded IP, marks this bridge's existing
// lights unreachable and returns without touching GC - a bridge blip must
// never look like "these lights were removed."
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

	lights, err := lighthue.FetchLights(ctx, hueBridge.Status.IP, b.ID, b.AppKey)
	if err != nil {
		logger.Error(err, "failed to fetch lights from bridge", "bridge", b.ID, "ip", hueBridge.Status.IP)
		p.markUnreachable(ctx, logger, b.ID)
		return
	}

	seen := make(map[string]bool, len(lights))
	for _, l := range lights {
		seen[l.ID] = true
		p.upsert(ctx, logger, l)
	}
	p.gc(ctx, logger, b.ID, seen)
}

// syncOne refreshes a single light's status, given its bridge and light
// ID - used for Trigger-driven out-of-band refreshes. Deliberately not a
// call into syncBridge (which fetches every light + every device on the
// bridge, and runs GC): none of that is wanted for "just recheck the one
// light we just changed."
func (p *Poller) syncOne(ctx context.Context, logger logr.Logger, bridgeID, lightID string) {
	var hueBridge lightsv1alpha1.HueBridge
	if err := p.Client.Get(ctx, client.ObjectKey{Name: bridges.ResourceName(bridgeID)}, &hueBridge); err != nil {
		return // nothing trustworthy to refresh from right now; the regular ticker will catch it later
	}
	if !hueBridge.Status.Reachable || hueBridge.Status.IP == "" {
		return
	}

	b, ok := bridges.FindByID(p.Bridges, bridgeID)
	if !ok {
		logger.Error(nil, "triggered sync for unknown bridge", "bridge", bridgeID)
		return
	}

	light, err := lighthue.FetchLight(ctx, hueBridge.Status.IP, b.AppKey, lightID)
	if err != nil {
		logger.Error(err, "failed to fetch light for triggered sync", "light", lightID)
		return
	}
	p.upsert(ctx, logger, light)
}

// upsert ensures a Light CR named after l.ID exists and carries l's
// current status. Object creation/labeling and the status subresource are
// deliberately two separate client calls - controller-runtime's
// CreateOrUpdate doesn't handle status subresources in the same pass.
func (p *Poller) upsert(ctx context.Context, logger logr.Logger, l lighthue.Light) {
	light := &lightsv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: l.ID}}

	_, err := controllerutil.CreateOrUpdate(ctx, p.Client, light, func() error {
		if light.Labels == nil {
			light.Labels = map[string]string{}
		}
		light.Labels[bridgeIDLabel] = l.BridgeID

		// Seed Spec from the just-fetched live state exactly once, at
		// creation. CreateOrUpdate's Get populates CreationTimestamp
		// before invoking this closure on an existing object, so a zero
		// CreationTimestamp reliably means "this call is creating a new
		// object" - never "updating an existing one." After this the
		// poller must never touch Spec again - see Reconciler for what
		// happens to it from here.
		if light.CreationTimestamp.IsZero() {
			light.Spec = lightsv1alpha1.LightSpec{
				Name:       l.Name,
				On:         l.On,
				Brightness: int32(l.Brightness),
				Color:      l.Color,
				ColorTempK: int32(l.ColorTempK),
			}
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to upsert light", "light", l.ID)
		return
	}

	light.Status = lightsv1alpha1.LightStatus{
		Name:        l.Name,
		BridgeID:    l.BridgeID,
		DeviceID:    l.DeviceID,
		On:          l.On,
		Brightness:  int32(l.Brightness),
		Color:       l.Color,
		ColorTempK:  int32(l.ColorTempK),
		FixtureType: l.FixtureType,
		Product:     l.Product,
		Model:       l.Model,
		Reachable:   true,
		LastSynced:  metav1.Now(),
	}
	if err := p.Client.Status().Update(ctx, light); err != nil {
		logger.Error(err, "failed to update light status", "light", l.ID)
	}
}

// markUnreachable flips status.reachable to false for every Light CR
// labeled with bridgeID, leaving the rest of their status (last-known
// on/brightness/etc.) untouched.
func (p *Poller) markUnreachable(ctx context.Context, logger logr.Logger, bridgeID string) {
	var list lightsv1alpha1.LightList
	if err := p.Client.List(ctx, &list, client.MatchingLabels{bridgeIDLabel: bridgeID}); err != nil {
		logger.Error(err, "failed to list lights to mark unreachable", "bridge", bridgeID)
		return
	}
	for i := range list.Items {
		light := &list.Items[i]
		if !light.Status.Reachable {
			continue
		}
		light.Status.Reachable = false
		if err := p.Client.Status().Update(ctx, light); err != nil {
			logger.Error(err, "failed to mark light unreachable", "light", light.Name)
		}
	}
}

// gc deletes Light CRs labeled with bridgeID whose name isn't in seen -
// only called for a bridge that was successfully polled this cycle, so
// "not in seen" reliably means "no longer on the bridge," not "bridge was
// unreachable."
func (p *Poller) gc(ctx context.Context, logger logr.Logger, bridgeID string, seen map[string]bool) {
	var list lightsv1alpha1.LightList
	if err := p.Client.List(ctx, &list, client.MatchingLabels{bridgeIDLabel: bridgeID}); err != nil {
		logger.Error(err, "failed to list lights for gc", "bridge", bridgeID)
		return
	}
	for i := range list.Items {
		light := &list.Items[i]
		if seen[light.Name] {
			continue
		}
		if err := p.Client.Delete(ctx, light); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete stale light", "light", light.Name)
		}
	}
}
