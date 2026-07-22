package switchcontroller

import (
	"testing"
	"time"

	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergedSwitchStatus(t *testing.T) {
	now := metav1.NewTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	older := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 11, 30, 0, 0, time.UTC)

	t.Run("polled event older than current keeps current's event fields", func(t *testing.T) {
		current := lightsv1alpha1.SwitchStatus{
			LastEvent:            "long_press",
			LastEventTime:        metav1.NewTime(newer),
			LastHandledEventTime: metav1.NewTime(newer),
		}
		polled := lighthue.Switch{Name: "Lounge", LastEvent: "short_release", LastEventTime: older}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "long_press" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want current's untouched (long_press, %v)", got.LastEvent, got.LastEventTime.Time, newer)
		}
		if !got.LastHandledEventTime.Time.Equal(newer) {
			t.Errorf("got LastHandledEventTime=%v, want preserved %v", got.LastHandledEventTime.Time, newer)
		}
		if got.Name != "Lounge" {
			t.Errorf("got Name=%q, want polled value Lounge", got.Name)
		}
	})

	t.Run("polled event newer than current advances event fields but preserves handled", func(t *testing.T) {
		current := lightsv1alpha1.SwitchStatus{
			LastEvent:            "short_release",
			LastEventTime:        metav1.NewTime(older),
			LastHandledEventTime: metav1.NewTime(older),
		}
		polled := lighthue.Switch{LastEvent: "long_press", LastEventTime: newer}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "long_press" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want advanced to (long_press, %v)", got.LastEvent, got.LastEventTime.Time, newer)
		}
		if !got.LastHandledEventTime.Time.Equal(older) {
			t.Errorf("got LastHandledEventTime=%v, want preserved %v (not advanced)", got.LastHandledEventTime.Time, older)
		}
	})

	t.Run("polled zero LastEventTime never fired doesn't clobber current", func(t *testing.T) {
		current := lightsv1alpha1.SwitchStatus{
			LastEvent:     "short_release",
			LastEventTime: metav1.NewTime(newer),
		}
		polled := lighthue.Switch{LastEvent: "", LastEventTime: time.Time{}}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "short_release" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want current's untouched", got.LastEvent, got.LastEventTime.Time)
		}
	})

	t.Run("always sets Reachable true and LastSynced", func(t *testing.T) {
		got := mergedSwitchStatus(lightsv1alpha1.SwitchStatus{}, lighthue.Switch{}, now)
		if !got.Reachable {
			t.Error("got Reachable=false, want true")
		}
		if !got.LastSynced.Time.Equal(now.Time) {
			t.Errorf("got LastSynced=%v, want %v", got.LastSynced.Time, now.Time)
		}
	})
}
