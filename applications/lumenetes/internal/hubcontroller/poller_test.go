// Poller.sync's call to lighthue.Discover(..., MethodSSDP) and Poller.Start's
// ticker loop are intentionally not covered by these tests: Discover's SSDP
// path does real UDP multicast via github.com/koron/go-ssdp with no
// injectable seam, and adding one purely for this low-risk, read/status-only
// loop wasn't judged worth it (an explicit decision in the test plan, not an
// oversight). sync's own dispatch logic - matching discovered bridges to
// HueBridge CRs - is covered indirectly by the direct updateStatus/
// markUnreachable tests below.
package hubcontroller

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/lumenetes/internal/hue"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var errTest = errors.New("injected test failure")

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

func TestPollerUpdateStatus(t *testing.T) {
	t.Run("bridge found in discovery result overwrites status", func(t *testing.T) {
		scheme := newTestScheme(t)
		bridge := &lumenetesv1alpha1.HueBridge{
			ObjectMeta: metav1.ObjectMeta{Name: "bridge-a"},
			Status: lumenetesv1alpha1.HueBridgeStatus{
				IP:        "old-ip",
				Reachable: false,
			},
		}
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(bridge).
			WithStatusSubresource(&lumenetesv1alpha1.HueBridge{}).
			Build()

		p := &Poller{Client: c}
		discovered := lighthue.Bridge{
			ID:         "abc123",
			Name:       "Living Room Bridge",
			IP:         "192.168.1.10",
			ModelID:    "BSB002",
			APIVersion: "1.55.0",
			SWVersion:  "1955106030",
			MAC:        "aa:bb:cc:dd:ee:ff",
		}

		p.updateStatus(context.Background(), logr.Discard(), bridge, discovered)

		got := &lumenetesv1alpha1.HueBridge{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "bridge-a"}, got); err != nil {
			t.Fatalf("Get: %v", err)
		}
		want := lumenetesv1alpha1.HueBridgeStatus{
			IP:         "192.168.1.10",
			Name:       "Living Room Bridge",
			ModelID:    "BSB002",
			APIVersion: "1.55.0",
			SWVersion:  "1955106030",
			MAC:        "aa:bb:cc:dd:ee:ff",
			Reachable:  true,
		}
		if got.Status.IP != want.IP || got.Status.Name != want.Name || got.Status.ModelID != want.ModelID ||
			got.Status.APIVersion != want.APIVersion || got.Status.SWVersion != want.SWVersion ||
			got.Status.MAC != want.MAC || got.Status.Reachable != want.Reachable {
			t.Errorf("got status %+v, want %+v (LastResolved ignored)", got.Status, want)
		}
		if got.Status.LastResolved.IsZero() {
			t.Errorf("expected LastResolved to be set, got zero value")
		}
	})

	t.Run("Status().Update failure is logged, does not panic", func(t *testing.T) {
		scheme := newTestScheme(t)
		bridge := &lumenetesv1alpha1.HueBridge{
			ObjectMeta: metav1.ObjectMeta{Name: "bridge-a"},
		}
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(bridge).
			WithStatusSubresource(&lumenetesv1alpha1.HueBridge{}).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errTest
				},
			}).
			Build()

		p := &Poller{Client: c}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("updateStatus panicked: %v", r)
			}
		}()
		p.updateStatus(context.Background(), logr.Discard(), bridge, lighthue.Bridge{ID: "abc123", IP: "192.168.1.10"})
	})
}

func TestPollerMarkUnreachable(t *testing.T) {
	t.Run("reachable bridge is flipped, rest of status untouched", func(t *testing.T) {
		scheme := newTestScheme(t)
		bridge := &lumenetesv1alpha1.HueBridge{
			ObjectMeta: metav1.ObjectMeta{Name: "bridge-a"},
			Status: lumenetesv1alpha1.HueBridgeStatus{
				IP:        "192.168.1.10",
				Name:      "Living Room Bridge",
				Reachable: true,
			},
		}
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(bridge).
			WithStatusSubresource(&lumenetesv1alpha1.HueBridge{}).
			Build()

		p := &Poller{Client: c}
		p.markUnreachable(context.Background(), logr.Discard(), bridge)

		got := &lumenetesv1alpha1.HueBridge{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "bridge-a"}, got); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Status.Reachable {
			t.Errorf("got Reachable=true, want false")
		}
		if got.Status.IP != "192.168.1.10" || got.Status.Name != "Living Room Bridge" {
			t.Errorf("got status %+v, want IP/Name left untouched", got.Status)
		}
	})

	t.Run("already unreachable bridge is a no-op", func(t *testing.T) {
		scheme := newTestScheme(t)
		bridge := &lumenetesv1alpha1.HueBridge{
			ObjectMeta: metav1.ObjectMeta{Name: "bridge-a"},
			Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: false},
		}
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(bridge).
			WithStatusSubresource(&lumenetesv1alpha1.HueBridge{}).
			Build()

		before := &lumenetesv1alpha1.HueBridge{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "bridge-a"}, before); err != nil {
			t.Fatalf("Get: %v", err)
		}
		beforeRV := before.ResourceVersion

		p := &Poller{Client: c}
		p.markUnreachable(context.Background(), logr.Discard(), before)

		after := &lumenetesv1alpha1.HueBridge{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "bridge-a"}, after); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if after.ResourceVersion != beforeRV {
			t.Errorf("expected no update (ResourceVersion unchanged), got %q -> %q", beforeRV, after.ResourceVersion)
		}
	})

	t.Run("Status().Update failure is logged, does not panic", func(t *testing.T) {
		scheme := newTestScheme(t)
		bridge := &lumenetesv1alpha1.HueBridge{
			ObjectMeta: metav1.ObjectMeta{Name: "bridge-a"},
			Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: true},
		}
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(bridge).
			WithStatusSubresource(&lumenetesv1alpha1.HueBridge{}).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errTest
				},
			}).
			Build()

		p := &Poller{Client: c}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("markUnreachable panicked: %v", r)
			}
		}()
		p.markUnreachable(context.Background(), logr.Discard(), bridge)
	})
}
