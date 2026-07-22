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

	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	"github.com/liamawhite/lights-controller/internal/eventstream"
	"github.com/liamawhite/lights-controller/internal/lightscontroller"
	"github.com/liamawhite/lights-controller/internal/switchcontroller"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// eventChannelBuffer sizes the buffered channels between eventstream.
// Streamer (one shared reader per bridge) and each controller's own
// EventConsumer goroutine - an internal implementation detail, not an
// operational knob, so it's a constant rather than a flag.
const eventChannelBuffer = 32

func main() {
	var (
		bridgesFile        string
		pollInterval       time.Duration
		healthProbeAddr    string
		resyncPeriod       time.Duration
		dryRun             bool
		switchPollInterval time.Duration
		leaderElectionID   = "lights-controller-leader"
	)
	flag.StringVar(&bridgesFile, "bridges-file", "/etc/lights-controller/bridges.json", "Path to the mounted bridges Secret (JSON array of {id, appKey})")
	flag.DurationVar(&pollInterval, "poll-interval", 30*time.Second, "How often to do a full poll sweep of bridges and sync Light status - a drift safety net behind the real-time eventstream path, not the primary sync mechanism")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "Address the health/readiness endpoints bind to")
	flag.DurationVar(&resyncPeriod, "resync-period", time.Minute, "How often the manager's cache does a full relist, forcing a re-reconcile of every Light in addition to reconciling immediately on every spec edit")
	flag.BoolVar(&dryRun, "dry-run", false, "If true, the Light reconciler only logs spec/status drift instead of enacting it against the bridge")
	flag.DurationVar(&switchPollInterval, "switch-poll-interval", 5*time.Minute, "How often to poll bridges for switch discovery/battery/reachability - the sub-second event path is handled by the eventstream, not this poller")
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

	poller := &lightscontroller.Poller{
		Client:       mgr.GetClient(),
		Bridges:      bridgeConfigs,
		PollInterval: pollInterval,
	}
	if err := mgr.Add(poller); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register poller: %v\n", err)
		os.Exit(1)
	}

	// MaxConcurrentReconciles > 1 as cheap throughput headroom for multiple
	// Lights enacting around the same time - each Reconcile is now just
	// fast in-cluster API calls plus a bounded bridge PUT, no blocking wait.
	if err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		For(&lightsv1alpha1.Light{}).
		Complete(&lightscontroller.Reconciler{
			Client:  mgr.GetClient(),
			Bridges: bridgeConfigs,
			DryRun:  dryRun,
		}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register light reconciler: %v\n", err)
		os.Exit(1)
	}

	switchPoller := &switchcontroller.Poller{
		Client:       mgr.GetClient(),
		Bridges:      bridgeConfigs,
		PollInterval: switchPollInterval,
	}
	if err := mgr.Add(switchPoller); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register switch poller: %v\n", err)
		os.Exit(1)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		For(&lightsv1alpha1.Switch{}).
		Complete(&switchcontroller.Reconciler{Client: mgr.GetClient()}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register switch reconciler: %v\n", err)
		os.Exit(1)
	}

	// One shared eventstream reader per bridge, publishing decoded events
	// onto channels each controller drains on its own goroutine - see
	// internal/eventstream's package doc for why the K8s writes are kept
	// off the socket-reading goroutine.
	buttonEvents := make(chan lighthue.ButtonEvent, eventChannelBuffer)
	lightEvents := make(chan lighthue.LightEvent, eventChannelBuffer)

	streamer := &eventstream.Streamer{
		Client:       mgr.GetClient(),
		Bridges:      bridgeConfigs,
		ButtonEvents: buttonEvents,
		LightEvents:  lightEvents,
	}
	if err := mgr.Add(streamer); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register eventstream streamer: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Add(&lightscontroller.EventConsumer{Client: mgr.GetClient(), Events: lightEvents}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register light event consumer: %v\n", err)
		os.Exit(1)
	}
	if err := mgr.Add(&switchcontroller.EventConsumer{Client: mgr.GetClient(), Events: buttonEvents}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register switch event consumer: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting lights-controller", "bridges", len(bridgeConfigs), "pollInterval", pollInterval, "resyncPeriod", resyncPeriod, "dryRun", dryRun, "switchPollInterval", switchPollInterval)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "manager exited with error: %v\n", err)
		os.Exit(1)
	}
}
