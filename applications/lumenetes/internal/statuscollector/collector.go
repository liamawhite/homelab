// Package statuscollector implements a prometheus.Collector that exposes
// every Light/Switch/Group/Scene/CircadianSchedule/HueBridge's live
// .status as Prometheus gauges, computed fresh from the manager's cached
// client on every scrape - no polling loop, no goroutine, no state of its
// own. All six types are already watched by existing reconcilers, so this
// adds no new informers/watches - each List below is served from the
// manager's already-synced cache.
//
// Only registered by cmd/lumenetes-controller (not cmd/hub-controller):
// its RBAC (see pkg/components/lumenetescontroller's ClusterRole) is the
// only one of the two that can read Light/Switch/Group/Scene/
// CircadianSchedule - hub-controller's ClusterRole only grants huebridges.
package statuscollector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Collector reads CR status straight from client on every scrape.
type Collector struct {
	Client client.Client
}

var _ prometheus.Collector = (*Collector)(nil)

// Describe intentionally sends nothing - an "unchecked" collector (see
// prometheus.Collector's own doc comment): the set of light/switch/group/
// scene/schedule/bridge names changes as CRs are created/GC'd, so there's
// no fixed descriptor set to declare up front. Same pattern
// kube-state-metrics itself uses for its own resource collectors.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {}

// Collect lists every relevant CR type and emits its status as metrics. A
// List failure for one type is logged and skipped rather than aborting the
// whole scrape - a transient error on one type shouldn't blank out every
// other metric family.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger := log.Log.WithName("statuscollector")

	c.collectLights(ctx, logger, ch)
	c.collectSwitches(ctx, logger, ch)
	c.collectGroups(ctx, logger, ch)
	c.collectScenes(ctx, logger, ch)
	c.collectCircadianSchedules(ctx, logger, ch)
	c.collectBridges(ctx, logger, ch)
}

// boolToFloat converts a bool status field to the 0/1 a Prometheus gauge
// needs.
func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
