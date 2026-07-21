// Command lights-controller syncs live Philips Hue light status into
// Light custom resources, and (unless --dry-run) enacts Light.Spec changes
// back onto the physical bridge - see internal/lightscontroller.Reconciler.
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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var (
		bridgesFile      string
		pollInterval     time.Duration
		healthProbeAddr  string
		resyncPeriod     time.Duration
		dryRun           bool
		enactCooldown    time.Duration
		leaderElectionID = "lights-controller-leader"
	)
	flag.StringVar(&bridgesFile, "bridges-file", "/etc/lights-controller/bridges.json", "Path to the mounted bridges Secret (JSON array of {id, appKey})")
	flag.DurationVar(&pollInterval, "poll-interval", 60*time.Second, "How often to poll bridges and sync Light status")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "Address the health/readiness endpoints bind to")
	flag.DurationVar(&resyncPeriod, "resync-period", time.Minute, "How often the manager's cache does a full relist, forcing a re-reconcile of every Light in addition to reconciling immediately on every spec edit")
	flag.BoolVar(&dryRun, "dry-run", false, "If true, the Light reconciler only logs spec/status drift instead of enacting it against the bridge")
	flag.DurationVar(&enactCooldown, "enact-cooldown", 30*time.Second, "Minimum time between enactment attempts for a given Light, regardless of how often it is reconciled")
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
		Cache: cache.Options{
			SyncPeriod: &resyncPeriod,
		},
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

	// Buffered so a burst of near-simultaneous enactments across different
	// lights doesn't block Reconcile - a full buffer just drops a
	// redundant trigger, which is harmless (syncOne per-light is
	// idempotent, and the next regular tick will catch up regardless).
	syncTrigger := make(chan lightscontroller.LightRef, len(bridgeConfigs))

	poller := &lightscontroller.Poller{
		Client:       mgr.GetClient(),
		Bridges:      bridgeConfigs,
		PollInterval: pollInterval,
		Trigger:      syncTrigger,
	}
	if err := mgr.Add(poller); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register poller: %v\n", err)
		os.Exit(1)
	}

	// MaxConcurrentReconciles > 1 since a successful (non-dry-run)
	// enactment blocks for up to 10s confirming convergence - without
	// this, Lights being enacted around the same time would serialize
	// behind each other's wait.
	if err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		For(&lightsv1alpha1.Light{}).
		Complete(&lightscontroller.Reconciler{
			Client:      mgr.GetClient(),
			Bridges:     bridgeConfigs,
			Cooldown:    enactCooldown,
			TriggerSync: syncTrigger,
			DryRun:      dryRun,
		}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register light reconciler: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting lights-controller", "bridges", len(bridgeConfigs), "pollInterval", pollInterval, "resyncPeriod", resyncPeriod, "dryRun", dryRun, "enactCooldown", enactCooldown)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "manager exited with error: %v\n", err)
		os.Exit(1)
	}
}
