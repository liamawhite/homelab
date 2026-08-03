package grafana

import _ "embed"

// nodeMetricsDashboard and podMetricsDashboard are ported verbatim from
// _migrateme/project/monitoring/dashboards - every panel/target's
// "datasource" field is {"type": "prometheus", "uid": "prometheus"}, a
// fixed literal rather than a real generated UID, so component.go's
// datasource provisioning deliberately pins `uid: prometheus` to match it
// exactly, with no dashboard JSON rewriting needed.
//
// coredns-metrics.json is deliberately NOT embedded here: it queries
// CoreDNS-specific metrics, and nothing in pkg/components/prometheus scrapes
// CoreDNS (pkg/components/dns only creates network *policy*, not a
// ServiceMonitor) - shipping it now would mean every panel permanently
// shows "No data" until a future CoreDNS-scraping addition exists.
//
//go:embed node-metrics.json
var nodeMetricsDashboard string

//go:embed pod-metrics.json
var podMetricsDashboard string

// networkMetricsDashboard is hand-authored (unlike node/pod-metrics above,
// there's no upstream dashboard being ported here), covering L3/network
// visibility from metrics already scraped elsewhere in this stack rather
// than Cilium's own cilium_*/hubble_* metrics (not enabled/scraped anywhere
// in this repo - see pkg/components/cilium): node-exporter's node_network_*
// netdev counters (per-node throughput/errors/drops), cadvisor's
// container_network_* counters (per-pod throughput/errors/drops, same
// interface="eth0" convention podMetricsDashboard's own network panel
// already uses), and one aggregate panel over ztunnel's
// istio_dns_requests_total for mesh-wide DNS query rate - deliberately just
// a single summary series here, since istioZtunnelDashboard already covers
// the per-pod DNS breakdown. Same hardcoded
// {"type": "prometheus", "uid": "prometheus"} datasource convention as
// every other dashboard here.
//
//go:embed network-metrics.json
var networkMetricsDashboard string

// The five Istio dashboards below are ported from istio/istio's
// manifests/addons/dashboards (master branch), with one deliberate edit:
// upstream has every panel/target resolve its datasource through a
// dashboard-level "datasource" template variable (type: datasource, query:
// "prometheus") rather than a fixed uid - removed here (along with the
// variable itself) in favor of the same hardcoded
// {"type": "prometheus", "uid": "prometheus"} nodeMetricsDashboard/
// podMetricsDashboard already use, so there's no picker and no
// auto-resolve step, just a fixed reference matching component.go's
// `uid: prometheus` datasource pin. They depend on pkg/components/istio's
// own PodMonitors (istiod, ztunnel, every waypoint's Envoy) for data - see
// that package's monitoring.go.
//
// istio-service-dashboard.json and istio-workload-dashboard.json query
// Istio's standard request-telemetry labels (reporter, destination_service,
// destination_workload, etc.). Their panels will only show data for
// traffic that actually routes through a waypoint (see
// pkg/components/istio/waypoint's own doc comment for what creates one) -
// most raw ambient pod-to-pod traffic never touches a waypoint at all, so
// is invisible to these two specifically (istio-mesh-dashboard.json and
// istio-ztunnel-dashboard.json cover that traffic instead, via ztunnel's
// own L4 metrics).
//
// Second deliberate edit, both dashboards: the "Reporter" filter (custom
// template var "qrep") upstream defaults to "destination" - the sidecar
// convention for server-side telemetry - with only "source"/"destination"
// as options at all. A pure-ambient, no-sidecar mesh like this one never
// reports either value; every request's reporter is "waypoint" instead
// (confirmed live: `count by (reporter) (istio_requests_total)` returns
// only reporter="waypoint"). Without this fix every panel gated by
// reporter="$qrep" would be permanently "No data" regardless of which of
// the two original options was selected. Added "waypoint" as a third
// option and made it the default; kept "source"/"destination" available
// too in case a future app adopts a client-side waypoint of its own.
//
//go:embed istio-control-plane-dashboard.json
var istioControlPlaneDashboard string

//go:embed istio-mesh-dashboard.json
var istioMeshDashboard string

//go:embed istio-service-dashboard.json
var istioServiceDashboard string

//go:embed istio-workload-dashboard.json
var istioWorkloadDashboard string

//go:embed istio-ztunnel-dashboard.json
var istioZtunnelDashboard string

// lumenetesDashboard is hand-authored (unlike every dashboard above, there's
// no upstream "lumenetes" dashboard to port or adapt), generated from a
// one-off script rather than typed by hand to keep the 24-column gridPos
// math and Grafana panel schema consistent - same hardcoded
// {"type": "prometheus", "uid": "prometheus"} datasource convention as
// every other dashboard here, no template variables. Covers both halves of
// pkg/components/lumenetescontroller's metrics.go scrape target: physical
// light/switch/group/scene/bridge status (from
// applications/lumenetes/internal/statuscollector) and controller
// operational health (applications/lumenetes/internal/metrics's bridge-poll/
// eventstream counters, plus a couple of controller-runtime's own built-in
// reconcile-error series). hub-controller contributes only
// lumenetes_bridge_poll_total{poller="hub_discovery"} and its own
// controller_runtime_* series - it has no light/switch/group RBAC, so no
// statuscollector data of its own.
//
//go:embed lumenetes-dashboard.json
var lumenetesDashboard string
