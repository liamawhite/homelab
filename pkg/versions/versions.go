// Package versions centralizes the pinned versions of components deployed
// by pkg/deploy.
package versions

const (
	KubeVip    = "v1.2.1"
	GatewayAPI = "v1.6.0"
	Istio      = "1.30.2"
	Longhorn   = "v1.9.1"
	Cilium     = "1.19.5"
	Tailscale  = "1.102.4"

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

// Application images are pinned by manifest digest. Publishing in an app repo
// does not change these pins; select a published digest here to deploy a release.
const (
	ParentalImage  = "ghcr.io/liamawhite/parental:sha-3dcea8ff43eb0e9e0d45aa8a71d17c0ad570340d@sha256:1deffd591a6534e2580d3ff776659e7eb5ea7927fa5cbec42b56c2c851b91ccc"
	ShoppingImage  = "ghcr.io/liamawhite/shopping:sha-19456badcbff123e941177156aadbdb9f04af66e@sha256:4a338a0efee6cad0932e267e93f2220fc6a9c8b8d59d936dcef56f94f4f527c4"
	WorkoutsImage  = "ghcr.io/liamawhite/workouts:sha-8f80a626646cd45bac0563ac8fdd12254eb317e1@sha256:f1b00675e1be282edb26526e4ba18a90b943cbdec6733cc0ce927e53452a6df4"
	LumenetesImage = "ghcr.io/liamawhite/lumenetes:sha-aec3b1aca27889178e5aeffdb41395200ee30ff2@sha256:d0305819a97bfce0322895dc4b3822ec7cf64e135bb5d998744b0ec272ddf797"
	DashboardImage = "ghcr.io/liamawhite/dashboard:sha-4466a24a839de62263644f18c881efe7f36558c1@sha256:d50ab15b2ec6988ace34c7f207ce8f177bf5b54bb88a84bcda60929cbed5f3d3"
)
