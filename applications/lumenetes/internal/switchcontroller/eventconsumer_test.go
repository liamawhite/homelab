package switchcontroller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/lumenetes/internal/hue"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestHandleEvent_SwitchDoesNotExistYet_SkippedSilently(t *testing.T) {
	c := newFakeClient(t)
	consumer := &EventConsumer{Client: c}

	consumer.handleEvent(context.Background(), logr.Discard(), lighthue.ButtonEvent{
		ButtonID: "not-yet-discovered",
		Event:    "short_release",
		Time:     time.Now(),
	})
	// Nothing to assert beyond "did not panic" - the Switch still doesn't
	// exist, and creating it isn't this consumer's job (Poller's).
	if err := c.Get(context.Background(), client.ObjectKey{Name: "not-yet-discovered"}, &lumenetesv1alpha1.Switch{}); err == nil {
		t.Errorf("switch was created, want handleEvent to skip silently when it doesn't exist yet")
	}
}

func TestHandleEvent_ExistingSwitch_PatchesLastEventFields(t *testing.T) {
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "btn-1"},
		Status: lumenetesv1alpha1.SwitchStatus{
			LastEvent:     "initial_press",
			LastEventTime: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			Battery:       80, // untouched by handleEvent - a plain field overwrite of only LastEvent/LastEventTime
		},
	}
	c := newFakeClient(t, sw)
	consumer := &EventConsumer{Client: c}

	evTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	consumer.handleEvent(context.Background(), logr.Discard(), lighthue.ButtonEvent{
		ButtonID: "btn-1",
		Event:    "long_press",
		Time:     evTime,
	})

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v", err)
	}
	if got.Status.LastEvent != "long_press" || !got.Status.LastEventTime.Time.Equal(evTime) {
		t.Errorf("got LastEvent=%q LastEventTime=%v, want (long_press, %v)", got.Status.LastEvent, got.Status.LastEventTime.Time, evTime)
	}
	if got.Status.Battery != 80 {
		t.Errorf("Battery = %d, want untouched (80)", got.Status.Battery)
	}
}

func TestHandleEvent_StatusUpdateFails_DoesNotPanic(t *testing.T) {
	sw := &lumenetesv1alpha1.Switch{ObjectMeta: metav1.ObjectMeta{Name: "btn-1"}}
	c := newFakeClientWithInterceptors(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, cl client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return errors.New("injected status update failure")
		},
	}, sw)
	consumer := &EventConsumer{Client: c}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleEvent panicked: %v", r)
		}
	}()
	consumer.handleEvent(context.Background(), logr.Discard(), lighthue.ButtonEvent{
		ButtonID: "btn-1",
		Event:    "short_release",
		Time:     time.Now(),
	})
}
