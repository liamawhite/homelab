// Package eventstream owns the persistent CLIP v2 SSE connection(s) to
// each paired bridge, publishing decoded button and light events onto
// channels for internal/switchcontroller and internal/lightscontroller to
// each consume on their own goroutine - decoupling "read the socket" from
// "write to Kubernetes" so a slow apiserver can never stall the read loop.
package eventstream

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	minReconnectBackoff = 1 * time.Second
	maxReconnectBackoff = 30 * time.Second
)

// Streamer is a manager.Runnable that keeps one persistent CLIP v2
// eventstream connection open per paired bridge - one goroutine per bridge
// (a single TCP stream can't be shared across bridges with different IPs),
// but one Streamer for the whole binary rather than one per controller.
// Decoded events are pushed onto ButtonEvents/LightEvents; each channel's
// consumer owns its own K8s writes on its own goroutine, so a slow
// apiserver never stalls this Streamer's socket reads.
type Streamer struct {
	Client       client.Client
	Bridges      []bridges.Config
	ButtonEvents chan<- lighthue.ButtonEvent
	LightEvents  chan<- lighthue.LightEvent
}

var (
	_ manager.Runnable               = (*Streamer)(nil)
	_ manager.LeaderElectionRunnable = (*Streamer)(nil)
)

// NeedLeaderElection ensures only the elected leader replica ever streams
// events.
func (s *Streamer) NeedLeaderElection() bool { return true }

// Start launches one goroutine per configured bridge, each independently
// connecting/reconnecting, and blocks until ctx is done. Must not return
// before ctx.Done() even with zero configured bridges - a Runnable
// returning early stops every other Runnable in the manager too.
func (s *Streamer) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	done := make(chan struct{}, len(s.Bridges))
	for _, b := range s.Bridges {
		b := b
		go func() {
			defer func() { done <- struct{}{} }()
			s.runBridge(ctx, logger, b)
		}()
	}

	if len(s.Bridges) == 0 {
		<-ctx.Done()
		return nil
	}
	for range s.Bridges {
		<-done
	}
	return nil
}

// runBridge connects to b's eventstream and, on any disconnect, reconnects
// with capped exponential backoff, re-resolving b's current IP each
// attempt (it can change between reconnects). A failure here never
// affects any other bridge's goroutine.
func (s *Streamer) runBridge(ctx context.Context, logger logr.Logger, b bridges.Config) {
	backoff := minReconnectBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		ip, ok := s.resolveIP(ctx, b.ID)
		if !ok {
			s.sleep(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		err := lighthue.StreamEvents(ctx, ip, b.AppKey,
			func(ev lighthue.ButtonEvent) { s.publishButton(ctx, ev) },
			func(ev lighthue.LightEvent) { s.publishLight(ctx, ev) },
		)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Error(err, "eventstream disconnected, reconnecting", "bridge", b.ID, "backoff", backoff)
			backoff = nextBackoff(backoff)
		} else {
			backoff = minReconnectBackoff
		}
		s.sleep(ctx, backoff)
	}
}

// publishButton/publishLight push onto the caller-provided channels,
// bailing out on ctx.Done() instead of blocking forever if the manager is
// shutting down and nothing is draining the channel anymore.
func (s *Streamer) publishButton(ctx context.Context, ev lighthue.ButtonEvent) {
	select {
	case s.ButtonEvents <- ev:
	case <-ctx.Done():
	}
}

func (s *Streamer) publishLight(ctx context.Context, ev lighthue.LightEvent) {
	select {
	case s.LightEvents <- ev:
	case <-ctx.Done():
	}
}

// resolveIP looks up bridgeID's current IP from its HueBridge CR - same
// lookup lightscontroller's Poller/Reconciler already use.
func (s *Streamer) resolveIP(ctx context.Context, bridgeID string) (string, bool) {
	var hueBridge lightsv1alpha1.HueBridge
	if err := s.Client.Get(ctx, client.ObjectKey{Name: bridges.ResourceName(bridgeID)}, &hueBridge); err != nil {
		return "", false
	}
	if !hueBridge.Status.Reachable || hueBridge.Status.IP == "" {
		return "", false
	}
	return hueBridge.Status.IP, true
}

func (s *Streamer) sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func nextBackoff(cur time.Duration) time.Duration {
	next := cur * 2
	if next > maxReconnectBackoff {
		return maxReconnectBackoff
	}
	return next
}
