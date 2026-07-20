// Command lights-controller syncs live Philips Hue light status into
// Light custom resources. Read-only reporting only for now - see
// applications/lights-controller's README-equivalent in the plan this was
// built from for the deferred write/control phase.
//
// Bridge discovery is NOT done here - see cmd/hub-controller. This binary
// reads each configured bridge's current IP from the HueBridge CR
// hub-controller maintains.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	"github.com/liamawhite/lights-controller/internal/lightscontroller"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var (
		bridgesFile      string
		pollInterval     time.Duration
		healthProbeAddr  string
		leaderElectionID = "lights-controller-leader"
	)
	flag.StringVar(&bridgesFile, "bridges-file", "/etc/lights-controller/bridges.json", "Path to the mounted bridges Secret (JSON array of {id, appKey})")
	flag.DurationVar(&pollInterval, "poll-interval", 60*time.Second, "How often to poll bridges and sync Light status")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "Address the health/readiness endpoints bind to")
	flag.Parse()

	// ctrl.Log.WithName(...) alone never attaches a real logging backend -
	// it's still just the unset DelegatingLogSink, so SetLogger(that) is a
	// no-op. Without a real sink, controller-runtime/client-go's
	// continuous logging (REST requests, informer sync, leader election)
	// queues in that sink indefinitely rather than being dropped or
	// written anywhere - confirmed live as a multi-GB/sec memory blowup
	// within seconds of Start(). zap.New(...) is the real sink.
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	logger := ctrl.Log.WithName("lights-controller")

	bridgeConfigs, err := bridges.Load(bridgesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load bridges file: %v\n", err)
		os.Exit(1)
	}

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

	poller := &lightscontroller.Poller{
		Client:       mgr.GetClient(),
		Bridges:      bridgeConfigs,
		PollInterval: pollInterval,
	}
	if err := mgr.Add(poller); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register poller: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting lights-controller", "bridges", len(bridgeConfigs), "pollInterval", pollInterval)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "manager exited with error: %v\n", err)
		os.Exit(1)
	}
}
