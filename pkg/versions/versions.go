// Package versions centralizes the pinned versions of components deployed
// by pkg/deploy.
package versions

const (
	KubeVip    = "v1.2.1"
	GatewayAPI = "v1.6.0"
	Istio      = "1.30.2"
	Longhorn   = "v1.9.1"
	Cilium     = "1.19.5"
	Tailscale  = "1.98.9"

	Prometheus         = "3.5.0"
	PrometheusOperator = "0.84.1"
	Alertmanager       = "0.28.1"
	Grafana            = "11.6.2"
	NodeExporter       = "1.9.1"
	KubeStateMetrics   = "2.14.0"
	// KubeRBACProxy is shared by prometheus-operator's and node-exporter's
	// kube-rbac-proxy sidecars.
	KubeRBACProxy = "0.19.1"
)
