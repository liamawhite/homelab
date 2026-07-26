// Package metrics holds the handful of custom operational metrics this
// module registers on top of controller-runtime's own built-in ones
// (controller_runtime_reconcile_total, workqueue_depth, client-go REST
// call metrics, etc. - already exported for free via
// sigs.k8s.io/controller-runtime/pkg/metrics.Registry, the same registry
// these are added to here). Covers the bridge-facing I/O controller-runtime
// has no visibility into itself: poll round trips and the realtime
// eventstream connection.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// BridgePollTotal covers every poll-style bridge round trip in this
	// module - lightscontroller's FetchLights, switchcontroller's
	// FetchSwitches, hubcontroller's SSDP Discover - via the "poller"
	// label rather than one metric name per poller.
	BridgePollTotal = promauto.With(ctrlmetrics.Registry).NewCounterVec(prometheus.CounterOpts{
		Name: "lumenetes_bridge_poll_total",
		Help: "Total bridge poll attempts, by bridge, poller, and result.",
	}, []string{"bridge_id", "poller", "result"})

	// BridgePollDurationSeconds times the bridge fetch/discovery call
	// itself, not the CR reconciliation work around it.
	BridgePollDurationSeconds = promauto.With(ctrlmetrics.Registry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lumenetes_bridge_poll_duration_seconds",
		Help:    "Duration of the bridge fetch/discovery call itself.",
		Buckets: prometheus.DefBuckets,
	}, []string{"bridge_id", "poller"})

	// EventstreamConnected reflects whether a bridge's CLIP v2 eventstream
	// connection is currently up - the realtime path lightscontroller/
	// switchcontroller's EventConsumers depend on; the poll-based metrics
	// above are the drift-safety-net path behind it.
	EventstreamConnected = promauto.With(ctrlmetrics.Registry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "lumenetes_eventstream_connected",
		Help: "Whether the CLIP v2 eventstream connection to a bridge is currently up (1) or down (0).",
	}, []string{"bridge_id"})

	// EventstreamReconnectsTotal counts every reconnect attempt after the
	// first connection to a bridge's eventstream.
	EventstreamReconnectsTotal = promauto.With(ctrlmetrics.Registry).NewCounterVec(prometheus.CounterOpts{
		Name: "lumenetes_eventstream_reconnects_total",
		Help: "Total reconnect attempts to a bridge's eventstream after the first connection.",
	}, []string{"bridge_id"})
)
