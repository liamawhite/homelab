package grafana

import _ "embed"

// nodeMetricsDashboard and podMetricsDashboard are ported verbatim from
// _migrateme/project/monitoring/dashboards - neither hardcodes a datasource
// UID (every panel/target's "datasource" field is the Grafana-default
// placeholder), so they render correctly against whatever datasource
// component.go provisions as default, with no rewriting needed.
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
