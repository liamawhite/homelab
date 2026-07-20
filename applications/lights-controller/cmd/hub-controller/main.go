// Command hub-controller discovers Philips Hue bridges via SSDP and
// publishes their current IP as HueBridge custom resources. Runs with
// HostNetwork: true (see pkg/components/hubcontroller) - SSDP's multicast
// group-join can't work from a normal pod under this cluster's CNI,
// confirmed live via `cilium monitor --type drop` (an "Unknown L4
// protocol" drop on the IGMP membership report, beneath the level any
// NetworkPolicy can act on).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/hubcontroller"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var (
		pollInterval     time.Duration
		discoveryTimeout time.Duration
		healthProbeAddr  string
		leaderElectionID = "hub-controller-leader"
	)
	flag.DurationVar(&pollInterval, "poll-interval", 60*time.Second, "How often to run an SSDP discovery round")
	flag.DurationVar(&discoveryTimeout, "discovery-timeout", 5*time.Second, "Timeout for each SSDP discovery round")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "Address the health/readiness endpoints bind to")
	flag.Parse()

	// See lights-controller/main.go's identical comment: ctrl.Log.WithName(...)
	// alone never attaches a real logging backend, and without one
	// client-go/controller-runtime's continuous logging queues in the
	// unset sink indefinitely rather than being dropped - a confirmed
	// multi-GB/sec memory blowup within seconds of Start().
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	logger := ctrl.Log.WithName("hub-controller")

	scheme := runtime.NewScheme()
	if err := lightsv1alpha1.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register API types: %v\n", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		HealthProbeBindAddress:  healthProbeAddr,
		LeaderElection:          true,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: os.Getenv("POD_NAMESPACE"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create manager: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register healthz check: %v\n", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register readyz check: %v\n", err)
		os.Exit(1)
	}

	poller := &hubcontroller.Poller{
		Client:       mgr.GetClient(),
		Timeout:      discoveryTimeout,
		PollInterval: pollInterval,
	}
	if err := mgr.Add(poller); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register poller: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting hub-controller", "pollInterval", pollInterval)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "manager exited with error: %v\n", err)
		os.Exit(1)
	}
}
