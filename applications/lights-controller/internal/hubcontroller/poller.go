// Package hubcontroller implements the poll-and-sync loop that keeps
// HueBridge custom resources' status in sync with live SSDP discovery
// results. The HueBridge objects themselves are created/deleted
// declaratively by pkg/components/hubcontroller (Pulumi), one per
// infra.yaml bridge entry - this package only ever reads and updates
// .status on an object that already exists.
package hubcontroller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Poller is a manager.Runnable that periodically discovers Hue bridges via
// SSDP and syncs their current IP (and other bridge info) onto the
// corresponding HueBridge custom resource's status, so lights-controller
// can reach them without doing any discovery of its own. Which bridges to
// sync comes from listing HueBridge CRs directly - pkg/components/
// hubcontroller (Pulumi) creates one per infra.yaml bridge entry, so that
// list is the live source of truth and needs no separate config file.
// Runs with HostNetwork: true (see pkg/components/hubcontroller) - SSDP's
// multicast group-join can't work from a normal pod under this cluster's
// CNI, confirmed live.
type Poller struct {
	Client       client.Client
	Timeout      time.Duration
	PollInterval time.Duration
}

var (
	_ manager.Runnable               = (*Poller)(nil)
	_ manager.LeaderElectionRunnable = (*Poller)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever writes
// HueBridge status - relevant now defensively, essential if replicas are
// ever raised above one.
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

// sync runs one SSDP discovery round, lists the current HueBridge CRs
// (the live source of truth for which bridges to sync - see the Poller
// doc comment), and syncs each one's status against the discovery
// result. SSDP has no cloud rate-limit exposure to guard against (unlike
// N-UPnP), so - unlike lights-controller's now-deleted IP resolver -
// there's no caching/cooldown layer here: every tick just discovers
// fresh.
func (p *Poller) sync(ctx context.Context, logger logr.Logger) {
	discovered, err := lighthue.Discover(ctx, p.Timeout, lighthue.MethodSSDP)
	if err != nil {
		logger.Error(err, "discovery failed")
	}
	byName := make(map[string]lighthue.Bridge, len(discovered))
	for _, b := range discovered {
		byName[bridges.ResourceName(b.ID)] = b
	}

	var list lightsv1alpha1.HueBridgeList
	if err := p.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list HueBridge CRs")
		return
	}
	for i := range list.Items {
		bridge := &list.Items[i]
		if found, ok := byName[bridge.Name]; ok {
			p.updateStatus(ctx, logger, bridge, found)
		} else {
			p.markUnreachable(ctx, logger, bridge)
		}
	}
}

// updateStatus syncs b's current info onto bridge's status.
func (p *Poller) updateStatus(ctx context.Context, logger logr.Logger, bridge *lightsv1alpha1.HueBridge, b lighthue.Bridge) {
	bridge.Status = lightsv1alpha1.HueBridgeStatus{
		IP:           b.IP,
		Name:         b.Name,
		ModelID:      b.ModelID,
		APIVersion:   b.APIVersion,
		SWVersion:    b.SWVersion,
		MAC:          b.MAC,
		Reachable:    true,
		LastResolved: metav1.Now(),
	}
	if err := p.Client.Status().Update(ctx, bridge); err != nil {
		logger.Error(err, "failed to update bridge status", "bridge", bridge.Name)
	}
}

// markUnreachable flips status.reachable to false for bridge, leaving the
// rest of its status (last-known IP/etc.) untouched.
func (p *Poller) markUnreachable(ctx context.Context, logger logr.Logger, bridge *lightsv1alpha1.HueBridge) {
	if !bridge.Status.Reachable {
		return
	}
	bridge.Status.Reachable = false
	if err := p.Client.Status().Update(ctx, bridge); err != nil {
		logger.Error(err, "failed to mark bridge unreachable", "bridge", bridge.Name)
	}
}
